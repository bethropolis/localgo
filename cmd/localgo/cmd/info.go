package cmd

import (
	"fmt"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/spf13/cobra"
)

var infojsonOutput bool

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show device information and configuration",
	RunE: func(cmd *cobra.Command, args []string) error {

		format := cli.FormatTable
		if infojsonOutput {
			format = cli.FormatJSON
		}

		writer := cli.NewOutputWriter(format)
		defer writer.Flush()

		deviceModel := "Unknown"
		if Cfg.DeviceModel != nil {
			deviceModel = *Cfg.DeviceModel
		}

		protocol := "HTTP"
		if Cfg.HttpsEnabled {
			protocol = "HTTPS"
		}

		info := cli.DeviceInfo{
			Alias:         Cfg.Alias,
			Version:       config.ProtocolVersion,
			DeviceModel:   deviceModel,
			DeviceType:    string(Cfg.DeviceType),
			Fingerprint:   Cfg.SecurityContext.CertificateHash,
			Port:          Cfg.Port,
			Protocol:      protocol,
			DownloadDir:   Cfg.DownloadDir,
			HasPin:        Cfg.PIN != "",
			MulticastAddr: fmt.Sprintf("%s:%d", Cfg.MulticastGroup, Cfg.Port),
		}

		return writer.WriteDeviceInfo(info)
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().BoolVar(&infojsonOutput, "json", false, "Output in JSON format")

	infoCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("info"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
