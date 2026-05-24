package cmd

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	sharefiles       []string
	shareport        int
	shareuseHTTP     bool
	sharepin         string
	sharealias       string
	shareautoAccept  bool
	sharenoClipboard bool
	sharehistory     string
	shareexecHook    string
	sharequiet       bool
	sharezip         bool
	shareconcurrency int
	sharemulticastiface string
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Share files so others can download them",
	RunE: func(cmd *cobra.Command, args []string) error {
		files := sharefiles

		if len(files) == 0 {
			return fmt.Errorf("file parameter is required (use --file)")
		}

		// Apply overrides
		if shareport > 0 {
			Cfg.Port = shareport
		}
		if shareuseHTTP {
			Cfg.HttpsEnabled = false
		}
		if sharepin != "" {
			Cfg.PIN = sharepin
		}
		if sharealias != "" {
			Cfg.Alias = sharealias
		}
		if shareautoAccept {
			Cfg.AutoAccept = true
		}
		if sharenoClipboard {
			Cfg.NoClipboard = true
		}
		if sharehistory != "" {
			Cfg.HistoryFile = sharehistory
		}
		if shareexecHook != "" {
			Cfg.ExecHook = shareexecHook
		}
		if sharequiet {
			Cfg.Quiet = true
		}
		if shareconcurrency > 0 {
			Cfg.Concurrency = shareconcurrency
		}
		if sharemulticastiface != "" {
			Cfg.MulticastInterface = sharemulticastiface
		}

		protocol := "HTTPS"
		if !Cfg.HttpsEnabled {
			protocol = "HTTP"
		}

		if !sharequiet {
			cli.PrintHeader("Starting LocalGo Web Share")
			cli.PrintInfo("Alias: %s", Cfg.Alias)
			cli.PrintInfo("Protocol: %s", protocol)
			cli.PrintInfo("Port: %d", Cfg.Port)
		}

	// Verify and prepare files
		filesMap := make(map[string]model.FileDto)
		pathsMap := make(map[string]string)
		var tempFiles []string

		for _, file := range files {
			fileInfo, err := os.Stat(file)
			if err != nil {
				return fmt.Errorf("file not found: %s", err)
			}

			// Reject directories that won't be zipped
			if fileInfo.IsDir() && !sharezip {
				return fmt.Errorf("cannot share directory '%s' without --zip flag", file)
			}

			// Zip directory if requested
			if fileInfo.IsDir() && sharezip {
				zipPath, err := zipDirToTemp(file)
				if err != nil {
					return fmt.Errorf("failed to zip directory %s: %w", file, err)
				}
				tempFiles = append(tempFiles, zipPath)
				zipInfo, _ := os.Stat(zipPath)
				fileInfo = zipInfo
				file = zipPath
			}

			f, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("failed to open file for detection: %w", err)
			}
			buffer := make([]byte, 512)
			n, _ := f.Read(buffer)
			contentType := http.DetectContentType(buffer[:n])
			f.Close()

			modTime := fileInfo.ModTime().Format(time.RFC3339)
			id := uuid.NewString()

			displayName := filepath.Base(file)
			if sharezip && strings.HasSuffix(file, ".zip") {
				displayName = filepath.Base(file[:len(file)-4]) + ".zip"
			}

			fileDto := model.FileDto{
				ID:       id,
				FileName: displayName,
				Size:     fileInfo.Size(),
				FileType: contentType,
				Metadata: &model.FileMetadata{
					Modified: &modTime,
				},
			}

			filesMap[id] = fileDto
			pathsMap[id] = file
			if !sharequiet {
				cli.PrintInfo("Sharing: %s (%s)", displayName, cli.FormatBytes(fileInfo.Size()))
			}
		}

		defer func() {
			for _, f := range tempFiles {
				os.Remove(f)
			}
		}()

		// Create server
		srv := server.NewServer(Cfg, zap.S())
		sendService := srv.GetSendService()

		// Register files in session
		_, err := sendService.CreateSession(filesMap, pathsMap)
		if err != nil {
			return fmt.Errorf("failed to create send session: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// Initialize discovery service with Download: true
		discoverySvcConfig := discovery.DefaultServiceConfig()
		discoverySvcConfig.MulticastConfig.Port = Cfg.Port
		discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", Cfg.MulticastGroup, Cfg.Port)
		discoverySvcConfig.MulticastConfig.InterfaceName = Cfg.MulticastInterface
		multicastDto := Cfg.ToMulticastDto(true)

		multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, zap.S())
		httpDiscoverer := discovery.NewHTTPDiscovery(nil, Cfg.ToRegisterDto(), nil, zap.S())
		multicast.SetHTTPDiscoverer(httpDiscoverer)

		peerCache := discovery.NewPeerCache(zap.S())
		multicast.SetPeerCache(peerCache)

		discoverySvc := discovery.NewService(discoverySvcConfig, multicast, zap.S())
		discoverySvc.SetPeerCache(peerCache)

		// Start server first
		serverErrChan := make(chan error, 1)
		serverReadyChan := make(chan struct{}, 1)
		go func() {
			serverErrChan <- srv.Start(ctx, serverReadyChan)
		}()

		// Wait for server to be ready
		select {
		case err := <-serverErrChan:
			return fmt.Errorf("server failed: %w", err)
		case <-serverReadyChan:
		}

		// Start discovery AFTER server is ready
		err = discoverySvc.Start(ctx, Cfg.Alias, Cfg.Port, Cfg.SecurityContext.CertificateHash, Cfg.DeviceType, Cfg.DeviceModel, Cfg.HttpsEnabled)
		if err != nil {
			return fmt.Errorf("discovery service failed: %w", err)
		}

		if !sharequiet {
			cli.PrintSuccess("Server ready! Waiting for connections...")
			cli.PrintWarning("Press Ctrl+C to stop sharing")
		}

		// Wait for server to finish
		if err := <-serverErrChan; err != nil {
			return fmt.Errorf("server failed: %w", err)
		}

		discoverySvc.Stop()
		if !sharequiet {
			cli.PrintInfo("Web share stopped")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(shareCmd)
	shareCmd.Flags().StringSliceVar(&sharefiles, "file", []string{}, "File or directory to share")
	shareCmd.Flags().IntVar(&shareport, "port", 0, "Port to run the server on")
	shareCmd.Flags().BoolVar(&shareuseHTTP, "http", false, "Use HTTP instead of HTTPS")
	shareCmd.Flags().StringVar(&sharepin, "pin", "", "PIN for authentication")
	shareCmd.Flags().StringVar(&sharealias, "alias", "", "Device alias")
	shareCmd.Flags().BoolVar(&shareautoAccept, "auto-accept", false, "Auto-accept incoming files")
	shareCmd.Flags().BoolVar(&sharenoClipboard, "no-clipboard", false, "Save text as file")
	shareCmd.Flags().StringVar(&sharehistory, "history", "", "Path to history file")
	shareCmd.Flags().StringVar(&shareexecHook, "exec", "", "Shell command to run")
	shareCmd.Flags().BoolVar(&sharequiet, "quiet", false, "Quiet mode")
	shareCmd.Flags().BoolVar(&sharezip, "zip", false, "Zip directories before sharing")
	shareCmd.Flags().IntVar(&shareconcurrency, "concurrency", 0, "Max parallel uploads (0 = use default)")
	shareCmd.Flags().StringVar(&sharemulticastiface, "iface", "", "Multicast network interface name")

	shareCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("share"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}

func zipDirToTemp(dir string) (string, error) {
	baseName := filepath.Base(dir)
	if baseName == "." || baseName == "/" {
		baseName = "archive"
	}
	zipPath, err := os.CreateTemp("", "localgo-"+baseName+"-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp zip: %w", err)
	}
	zipPath.Close()
	zipPathName := zipPath.Name()

	fZ, err := os.OpenFile(zipPathName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		os.Remove(zipPathName)
		return "", fmt.Errorf("failed to reopen temp zip: %w", err)
	}
	zipWriter := zip.NewWriter(fZ)

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			rel = info.Name()
		}
		rel = filepath.ToSlash(rel)

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate

		w, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		f.Close()
		return err
	})
	if err != nil {
		zipWriter.Close()
		os.Remove(zipPathName)
		return "", err
	}

	if err := zipWriter.Close(); err != nil {
		os.Remove(zipPathName)
		return "", err
	}

	return zipPathName, nil
}
