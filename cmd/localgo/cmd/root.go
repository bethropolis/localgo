package cmd

import (
	"fmt"
	"os"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var versionFlag bool

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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			help.ShowVersion(Version, GitCommit, BuildDate)
			os.Exit(0)
		}

		logger := logging.Init(Verbose, JSONOutput)

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

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&versionFlag, "version", false, "Show version information")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/localgo/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&JSONOutput, "json", false, "Enable JSON log output")

	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		help.ShowMainUsage()
	})
}
