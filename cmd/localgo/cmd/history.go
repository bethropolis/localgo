package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/history"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	historyLimit int
	historyClear bool
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show file transfer history log",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := Cfg.HistoryFile
		if path == "" {
			path = history.DefaultPath()
		}

		if historyClear {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				cli.PrintInfo("History log is already empty.")
				return nil
			}
			err := os.Remove(path)
			if err != nil {
				return fmt.Errorf("failed to clear history: %w", err)
			}
			cli.PrintSuccess("Transfer history cleared successfully.")
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				cli.PrintInfo("No transfer history found.")
				return nil
			}
			return fmt.Errorf("failed to open history file: %w", err)
		}
		defer file.Close()

		var entries []history.Entry
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var entry history.Entry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
				entries = append(entries, entry)
			}
		}

		if len(entries) == 0 {
			cli.PrintInfo("No transfer history found.")
			return nil
		}

		// Limit the view to the last N entries
		start := 0
		if len(entries) > historyLimit {
			start = len(entries) - historyLimit
		}
		displayEntries := entries[start:]

		// Reverse order so the newest items show up first
		for i, j := 0, len(displayEntries)-1; i < j; i, j = i+1, j-1 {
			displayEntries[i], displayEntries[j] = displayEntries[j], displayEntries[i]
		}

		titleStyle := cli.HeaderStyle.Padding(0, 1).MarginBottom(1)
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
		rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

		fmt.Println(titleStyle.Render(cli.IconFolderOpen+"  File Transfer History") + "\n")

		// Column Width definitions
		colWidths := []int{12, 16, 25, 10, 12} // Time, Sender, File, Size, Status

		// Print Header
		fmt.Printf("%s  %s  %s  %s  %s\n",
			padRight(headerStyle.Render("TIME"), colWidths[0]),
			padRight(headerStyle.Render("SENDER"), colWidths[1]),
			padRight(headerStyle.Render("FILE NAME"), colWidths[2]),
			padRight(headerStyle.Render("SIZE"), colWidths[3]),
			padRight(headerStyle.Render("STATUS"), colWidths[4]),
		)
		fmt.Println(mutedStyle.Render(strings.Repeat("-", 80)))

		for _, entry := range displayEntries {
			senderAlias := entry.SenderAlias
			if Cfg.Private {
				senderAlias = cli.AnonymizeString(entry.SenderAlias)
			}
			tStr := entry.Timestamp.Local().Format("01-02 15:04")

			statusColored := entry.Status
			switch entry.Status {
			case "received":
				statusColored = cli.SuccessStyle.Render("Received")
			case "clipboard":
				statusColored = cli.InfoStyle.Render("Clipboard")
			case "failed":
				statusColored = cli.ErrorStyle.Render("Failed")
			}

			fmt.Printf("%s  %s  %s  %s  %s\n",
				padRight(mutedStyle.Render(tStr), colWidths[0]),
				padRight(rowStyle.Render(cli.TruncateString(senderAlias, 14)), colWidths[1]),
				padRight(rowStyle.Render(cli.TruncateString(entry.FileName, 23)), colWidths[2]),
				padRight(rowStyle.Render(cli.FormatBytes(entry.FileSize)), colWidths[3]),
				padRight(statusColored, colWidths[4]),
			)
		}
		return nil
	},
}

func init() {
	historyCmd.Flags().IntVar(&historyLimit, "limit", 10, "Maximum number of entries to display")
	historyCmd.Flags().BoolVar(&historyClear, "clear", false, "Clear all transfer history logs")
	rootCmd.AddCommand(historyCmd)
}
