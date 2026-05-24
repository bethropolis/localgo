package config

import (
	"strings"

	"github.com/spf13/viper"
)

func InitViper() *viper.Viper {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("$HOME/.config/localgo/")
	v.AddConfigPath("$HOME/.local/etc/localgo/")
	v.AddConfigPath(".")

	v.SetEnvPrefix("LOCALSEND")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	// Set defaults
	v.SetDefault("port", DefaultPort)
	v.SetDefault("multicast_group", DefaultMulticastGroup)
	v.SetDefault("concurrency", 4)
	// We'll handle DownloadDir default in LoadConfig since it depends on os.UserHomeDir

	_ = v.ReadInConfig() // ignore error if config file doesn't exist

	return v
}
