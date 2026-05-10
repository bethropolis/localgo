package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
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

		// Validate required parameters
		if len(files) == 0 {
			return fmt.Errorf("file parameter is required (use --file)")
		}
		if sendto == "" {
			return fmt.Errorf("target device parameter is required (use --to)")
		}

		// Check if files exist
		for _, file := range files {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s", file)
			}
		}

		// Apply overrides
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
		cli.PrintInfo("To: %s", sendto)
		cli.PrintInfo("From: %s", Cfg.Alias)

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sendtimeout)*time.Second)
		defer cancel()

		// Send files
		err := send.SendFiles(ctx, Cfg, files, sendto, sendport, zap.S())
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
	sendCmd.Flags().StringVar(&sendto, "to", "", "Target device alias")
	sendCmd.Flags().IntVar(&sendport, "port", 0, "Target device port")
	sendCmd.Flags().IntVar(&sendtimeout, "timeout", 30, "Send timeout in seconds")
	sendCmd.Flags().StringVar(&sendalias, "alias", "", "Sender alias")

	sendCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("send"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
