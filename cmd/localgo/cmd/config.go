package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage LocalGo configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.New()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME/.config/localgo/")
		v.AddConfigPath("$HOME/.local/etc/localgo/")

		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("failed to read config: %w", err)
			}
		}

		key := strings.ToLower(args[0])
		if !v.InConfig(key) && !v.IsSet(key) {
			return fmt.Errorf("key %q not found in config", key)
		}

		fmt.Println(v.GetString(key))
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.New()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME/.config/localgo/")
		v.AddConfigPath("$HOME/.local/etc/localgo/")

		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("failed to read config: %w", err)
			}
		}

		key := strings.ToLower(args[0])

		existingVal := v.Get(key)
		switch existingVal.(type) {
		case int, int64:
			val, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid integer value %q: %w", args[1], err)
			}
			v.Set(key, val)
		case bool:
			val, err := strconv.ParseBool(args[1])
			if err != nil {
				return fmt.Errorf("invalid boolean value %q: %w", args[1], err)
			}
			v.Set(key, val)
		case float64:
			val, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("invalid float value %q: %w", args[1], err)
			}
			v.Set(key, val)
		default:
			v.Set(key, args[1])
		}

		configPath := v.ConfigFileUsed()
		if configPath == "" {
			configPath = os.ExpandEnv("$HOME/.config/localgo/config.yaml")
		}

		if err := os.MkdirAll(strings.TrimSuffix(configPath, "/config.yaml"), 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		if err := v.WriteConfigAs(configPath); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("Set %s = %q in %s\n", key, args[1], configPath)
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all config values",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.New()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME/.config/localgo/")
		v.AddConfigPath("$HOME/.local/etc/localgo/")

		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("failed to read config: %w", err)
			}
		}

		settings := v.AllSettings()
		if len(settings) == 0 {
			fmt.Println("(no config file found)")
			return nil
		}

		for _, key := range v.AllKeys() {
			val := v.Get(key)
			if val == nil {
				continue
			}
			fmt.Printf("%-25s %v\n", key, val)
		}
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.New()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME/.config/localgo/")
		v.AddConfigPath("$HOME/.local/etc/localgo/")

		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("failed to read config: %w", err)
			}
		}

		path := v.ConfigFileUsed()
		if path == "" {
			fmt.Println("$HOME/.config/localgo/config.yaml")
		} else {
			fmt.Println(path)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configPathCmd)
	rootCmd.AddCommand(configCmd)
}
