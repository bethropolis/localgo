package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/send"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	sendfiles   []string
	sendto      string
	sendport    int
	sendtimeout int
	sendalias   string
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a file to another LocalGo device",
	RunE: func(cmd *cobra.Command, args []string) error {
		files := sendfiles

		if len(files) == 0 {
			file, err := cli.PickFiles()
			if err != nil {
				return fmt.Errorf("file selection canceled or failed: %w", err)
			}
			files = []string{file}
		}

		for _, file := range files {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s", file)
			}
		}

		target := sendto
		if target == "" {
			cli.PrintHeader("Looking for devices...")
			devices, err := discovery.DiscoverDevices(
				context.Background(),
				discovery.DefaultServiceConfig(),
				Cfg.Alias, Cfg.Port, Cfg.SecurityContext.CertificateHash,
				Cfg.DeviceModel, Cfg.HttpsEnabled,
			)
			if err != nil {
				return fmt.Errorf("discovery failed: %w", err)
			}
			if len(devices) == 0 {
				return fmt.Errorf("no devices found on the network")
			}

			selected := cli.PickDevice(devices)
			if selected == nil {
				return fmt.Errorf("no device selected")
			}
			target = selected.Alias
			sendport = selected.Port
		}

		if sendalias != "" {
			Cfg.Alias = sendalias
		}

		cli.PrintHeader(fmt.Sprintf("Sending %d files", len(files)))
		for _, file := range files {
			fileInfo, err := os.Stat(file)
			if err == nil {
				cli.PrintInfo("- %s (%s)", filepath.Base(file), cli.FormatBytes(fileInfo.Size()))
			}
		}
		cli.PrintInfo("To: %s", target)
		cli.PrintInfo("From: %s", Cfg.Alias)

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
	sendCmd.Flags().StringVar(&sendto, "to", "", "Target device alias (omit to pick interactively)")
	sendCmd.Flags().IntVar(&sendport, "port", 0, "Target device port")
	sendCmd.Flags().IntVar(&sendtimeout, "timeout", 30, "Send timeout in seconds")
	sendCmd.Flags().StringVar(&sendalias, "alias", "", "Sender alias")

	sendCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("send"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
