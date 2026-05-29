package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/clipboard"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/bethropolis/localgo/pkg/send"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	sendfiles       []string
	sendip          string
	sendto          string
	sendport        int
	sendtimeout     int
	sendalias       string
	sendconcurrency int
	sendmulticastiface string
	sendclipboard   bool
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a file to another LocalGo device",
	RunE: func(cmd *cobra.Command, args []string) error {
		files := sendfiles

		// Interactive fallback: if no files specified, check clipboard for text content
		if len(files) == 0 && !sendclipboard {
			if clipboard.Available() {
				text, err := clipboard.Read()
				if err == nil && strings.TrimSpace(text) != "" {
					preview := strings.ReplaceAll(text, "\n", " ")
					if len(preview) > 50 {
						preview = preview[:50] + "…"
					}

					var useClip bool = true
					form := huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title("No files specified. Send clipboard content instead?").
								Description(fmt.Sprintf("Current clipboard: %q", preview)).
								Value(&useClip),
						),
					).WithTheme(huh.ThemeCharm())

					if err := form.Run(); err == nil && useClip {
						sendclipboard = true
					}
				}
			}
		}

		if sendclipboard {
			text, err := clipboard.Read()
			if err != nil {
				return fmt.Errorf("failed to read from clipboard: %w", err)
			}
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("clipboard is empty or does not contain text")
			}

			tempFile, err := os.CreateTemp("", "localgo-clip-*.txt")
			if err != nil {
				return fmt.Errorf("failed to create temporary file for clipboard: %w", err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.WriteString(text); err != nil {
				tempFile.Close()
				return fmt.Errorf("failed to write clipboard text: %w", err)
			}
			tempFile.Close()
			files = []string{tempFile.Name()}
		}

		if len(files) == 0 {
			return fmt.Errorf("no file specified: use positional args, --file flag, or --clipboard")
		}

		for _, file := range files {
			if _, err := os.Stat(file); os.IsNotExist(err) && !sendclipboard {
				return fmt.Errorf("file not found: %s", file)
			}
		}

		// Direct send via --ip: skip discovery entirely
		if sendip != "" {
			host, portStr, err := net.SplitHostPort(sendip)
			if err != nil {
				// No port specified; treat whole string as host
				host = sendip
				portStr = ""
			}
			parsedIP := net.ParseIP(host)
			if parsedIP == nil {
				return fmt.Errorf("invalid IP address: %s", host)
			}

			port := sendport
			if portStr != "" {
				p, err := strconv.Atoi(portStr)
				if err != nil {
					return fmt.Errorf("invalid port in --ip: %w", err)
				}
				port = p
			}
			if port == 0 {
				port = Cfg.Port
			}

			device := &model.Device{
				Alias: host,
				IP:    parsedIP.String(),
				Port:  port,
			}

			if sendalias != "" {
				Cfg.Alias = sendalias
			}
			if sendconcurrency > 0 {
				Cfg.Concurrency = sendconcurrency
			}

			cli.PrintHeader(fmt.Sprintf("Sending %d files", len(files)))
			for _, file := range files {
				fileInfo, err := os.Stat(file)
				if err == nil {
					cli.PrintInfo("- %s (%s)", filepath.Base(file), cli.FormatBytes(fileInfo.Size()))
				}
			}
			cli.PrintInfo("To: %s:%d", host, port)
			fromAlias := Cfg.Alias
			if Cfg.Private {
				fromAlias = "Anonymous"
			}
			cli.PrintInfo("From: %s", fromAlias)

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sendtimeout)*time.Second)
			defer cancel()

			if err := send.SendToDevice(ctx, Cfg, device, files, zap.S()); err != nil {
				return fmt.Errorf("failed to send files: %w", err)
			}

			cli.PrintSuccess("Files sent successfully!")
			return nil
		}

		target := sendto
		if target == "" {
			sendConfig := discovery.DefaultServiceConfig()
			sendConfig.MulticastConfig.InterfaceName = Cfg.MulticastInterface

			var devices []*model.Device
			var discErr error

			_ = spinner.New().
				Title("Looking for devices on your network (multicast)...").
				Action(func() {
					devices, discErr = discovery.DiscoverDevices(
						context.Background(),
						sendConfig,
						Cfg.Alias, Cfg.Port, Cfg.SecurityContext.CertificateHash,
						Cfg.DeviceModel, Cfg.HttpsEnabled,
					)
				}).
				Run()

			if discErr != nil {
				return fmt.Errorf("discovery failed: %w", discErr)
			}

			// Unicast fallback: if multicast finds nothing, automatically scan subnets
			if len(devices) == 0 {
				localIPs, ipErr := network.GetLocalIPAddresses()
				if ipErr == nil && len(localIPs) > 0 {
					_ = spinner.New().
						Title("No devices via multicast. Scanning local subnets...").
						Action(func() {
							registerDto := Cfg.ToRegisterDto()
							httpFallback := discovery.NewHTTPDiscovery(nil, registerDto, nil, nil)

							var ips []net.IP
							for _, ip := range localIPs {
								ips = append(ips, network.GetSubnetIPs(ip)...)
							}
							ips = append(ips, net.ParseIP("127.0.0.1"))

							scanCtx, cancelScan := context.WithTimeout(context.Background(), 10*time.Second)
							defer cancelScan()

							devices, _ = httpFallback.ScanNetwork(scanCtx, ips, Cfg.Port)
						}).
						Run()
				}
			}

			if len(devices) == 0 {
				return fmt.Errorf("no devices found on the network via multicast or subnet scan")
			}

			selected := cli.PickDevice(devices, Cfg.Private)
			if selected == nil {
				return fmt.Errorf("no device selected")
			}
			target = selected.Alias
			sendport = selected.Port
		}

		if sendalias != "" {
			Cfg.Alias = sendalias
		}
		if sendconcurrency > 0 {
			Cfg.Concurrency = sendconcurrency
		}
		if sendmulticastiface != "" {
			Cfg.MulticastInterface = sendmulticastiface
		}

		cli.PrintHeader(fmt.Sprintf("Sending %d files", len(files)))
		for _, file := range files {
			fileInfo, err := os.Stat(file)
			if err == nil {
				cli.PrintInfo("- %s (%s)", filepath.Base(file), cli.FormatBytes(fileInfo.Size()))
			}
		}
		cli.PrintInfo("To: %s", target)
		fromAlias := Cfg.Alias
		if Cfg.Private {
			fromAlias = "Anonymous"
		}
		cli.PrintInfo("From: %s", fromAlias)

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sendtimeout)*time.Second)
		defer cancel()

		err := send.SendFiles(ctx, Cfg, files, target, sendport, zap.S())
		if err != nil {
			return fmt.Errorf("failed to send files: %w", err)
		}

		cli.PrintSuccess("Files sent successfully!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringSliceVar(&sendfiles, "file", []string{}, "File or directory to send")
	sendCmd.Flags().StringVar(&sendip, "ip", "", "Target device IP (with optional :port, skips discovery)")
	sendCmd.Flags().StringVar(&sendto, "to", "", "Target device alias (omit to pick interactively)")
	sendCmd.Flags().IntVar(&sendport, "port", 0, "Target device port")
	sendCmd.Flags().IntVar(&sendtimeout, "timeout", 30, "Send timeout in seconds")
	sendCmd.Flags().StringVar(&sendalias, "alias", "", "Sender alias")
	sendCmd.Flags().IntVar(&sendconcurrency, "concurrency", 0, "Max parallel uploads (0 = use default)")
	sendCmd.Flags().StringVar(&sendmulticastiface, "iface", "", "Multicast network interface name")
	sendCmd.Flags().BoolVarP(&sendclipboard, "clipboard", "c", false, "Send current system clipboard text directly")

	sendCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("send"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
