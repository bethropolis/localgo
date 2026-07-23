package cmd

import (
	"fmt"
	"os"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/clipboard"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	versionFlag bool
	privateMode  bool
	noColor     bool
)

var (
	cfgFile    string
	Verbose    bool
	JSONOutput bool
	Cfg        *config.Config
	ViperCfg   *viper.Viper
)

var rootCmd = &cobra.Command{
	Use:   "localgo",
	Short: "LocalGo - LocalSend v2.1 Protocol Implementation",
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag {
			help.ShowVersion(Version, GitCommit, BuildDate)
			return
		}
		cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			help.ShowVersion(Version, GitCommit, BuildDate)
			os.Exit(0)
		}

		if noColor || os.Getenv("NO_COLOR") != "" {
			noColor = true
		}

		logger := logging.Init(Verbose, JSONOutput, noColor)

		ViperCfg = config.InitViper()
		if cfgFile != "" {
			ViperCfg.SetConfigFile(cfgFile)
			if err := ViperCfg.ReadInConfig(); err != nil {
				zap.S().Warnf("Failed to read config file: %v", err)
			}
		}

		var err error
		Cfg, err = config.LoadConfig(ViperCfg, logger)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		if Cfg.SecurityContext == nil {
			return fmt.Errorf("security context is missing after loading config")
		}

		if privateMode {
			Cfg.Private = true
		}

		if Cfg.ClipboardWriteCmd != "" || Cfg.ClipboardReadCmd != "" {
			clipboard.OverrideProvider(Cfg.ClipboardWriteCmd, Cfg.ClipboardReadCmd)
		}
		if Cfg.NotificationCmd != "" {
			cli.SetNotificationCmd(Cfg.NotificationCmd)
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&versionFlag, "version", "v", false, "Show version information")
	rootCmd.PersistentFlags().BoolVarP(&privateMode, "private", "p", false, "Hide device identity (alias, model) during discovery and transfer")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/localgo/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&JSONOutput, "json", false, "Enable JSON log output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		help.ShowMainUsage()
	})
}
