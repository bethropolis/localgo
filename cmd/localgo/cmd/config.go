package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/spf13/cobra"
)

// configKey describes a known config key with its type and valid values.
type configKey struct {
	typ     string      // "string", "int", "bool", "enum", "slice"
	enums   []string    // valid values for enum type
	intMin  int         // minimum for int type
	intMax  int         // maximum for int type
	defVal  interface{} // default value
}

var knownConfigKeys = map[string]configKey{
	"alias":                      {typ: "string"},
	"port":                       {typ: "int", intMin: 1, intMax: 65535, defVal: 53317},
	"multicast_group":            {typ: "string", defVal: "224.0.0.167"},
	"device_model":               {typ: "string"},
	"device_type":                {typ: "enum", enums: []string{"desktop", "mobile", "headless", "server"}},
	"auto_accept":                {typ: "bool"},
	"no_clipboard":               {typ: "bool"},
	"quiet":                      {typ: "bool"},
	"history":                    {typ: "string"},
	"exec":                       {typ: "string"},
	"concurrency":                {typ: "int", intMin: 1, intMax: 32, defVal: 4},
	"shell":                      {typ: "string"},
	"multicast_interface":        {typ: "string"},
	"discovery_strategy":         {typ: "enum", enums: []string{"full", "fast"}},
	"file_conflict_resolution":   {typ: "enum", enums: []string{"rename", "overwrite", "skip"}},
	"bind_address":               {typ: "string"},
	"static_peers":               {typ: "slice"},
	"trusted_fingerprints":       {typ: "slice"},
	"clipboard_write_cmd":        {typ: "string"},
	"clipboard_read_cmd":         {typ: "string"},
	"tls_cert":                   {typ: "string"},
	"tls_key":                    {typ: "string"},
	"notification_cmd":           {typ: "string"},
	"force_http":                 {typ: "bool"},
	"download_dir":               {typ: "string"},
	"max_body_size":              {typ: "int", intMin: 0, intMax: 1 << 30},
	"security_dir":               {typ: "string"},
}

// closeMatches returns keys whose Levenshtein distance is <= 2.
func closeMatches(input string, candidates []string) []string {
	var matches []string
	for _, c := range candidates {
		if levenshtein(input, c) <= 2 {
			matches = append(matches, c)
		}
	}
	return matches
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min3(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}
	return d[la][lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func knownKeyNames() []string {
	names := make([]string, 0, len(knownConfigKeys))
	for k := range knownConfigKeys {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func validateKey(key string) (configKey, error) {
	ck, ok := knownConfigKeys[key]
	if !ok {
		suggestions := closeMatches(key, knownKeyNames())
		if len(suggestions) > 0 {
			return ck, fmt.Errorf("unknown config key %q; did you mean %s?", key, strings.Join(suggestions, ", "))
		}
		return ck, fmt.Errorf("unknown config key %q", key)
	}
	return ck, nil
}

func validateValue(ck configKey, key, raw string) (interface{}, error) {
	switch ck.typ {
	case "string":
		return raw, nil
	case "int":
		val, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q", raw)
		}
		if val < ck.intMin || val > ck.intMax {
			return nil, fmt.Errorf("value %d out of range [%d, %d]", val, ck.intMin, ck.intMax)
		}
		return val, nil
	case "bool":
		val, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean %q (use true/false)", raw)
		}
		return val, nil
	case "enum":
		for _, e := range ck.enums {
			if strings.EqualFold(raw, e) {
				return e, nil
			}
		}
		return nil, fmt.Errorf("invalid value %q; valid values: %s", raw, strings.Join(ck.enums, ", "))
	case "slice":
		return nil, fmt.Errorf("use 'config add %s' or 'config remove %s' to manage list values", key, key)
	}
	return raw, nil
}

func newConfigSource() *config.Source {
	src := config.LoadSource()
	for key, ck := range knownConfigKeys {
		if ck.defVal != nil {
			src.SetDefault(key, ck.defVal)
		}
	}
	return src
}

// originFor reports the source of a key's value: file, env, or default.
func originFor(src *config.Source, key string) (string, bool) {
	if src.InFile(key) {
		return "[file]", true
	}
	if _, ok := os.LookupEnv("LOCALSEND_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))); ok {
		return "[env]", true
	}
	if ck, ok := knownConfigKeys[key]; ok && ck.defVal != nil {
		return "[default]", true
	}
	return "", false
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage LocalGo configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := newConfigSource()
		key := strings.ToLower(args[0])

		if _, err := validateKey(key); err != nil {
			return err
		}

		if !src.IsSet(key) {
			return fmt.Errorf("key %q not set", key)
		}

		fmt.Println(src.GetString(key))
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := newConfigSource()
		key := strings.ToLower(args[0])

		ck, err := validateKey(key)
		if err != nil {
			return err
		}

		val, err := validateValue(ck, key, args[1])
		if err != nil {
			return err
		}

		src.Set(key, val)

		if err := src.Save(); err != nil {
			return err
		}

		fmt.Printf("Set %s = %v in %s\n", key, val, src.FilePath())
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all config values with origin",
	RunE: func(cmd *cobra.Command, args []string) error {
		src := newConfigSource()

		fmt.Printf("%-28s %-10s %s\n", "KEY", "ORIGIN", "VALUE")
		fmt.Println(strings.Repeat("-", 80))

		shown := false
		for _, key := range knownKeyNames() {
			origin, ok := originFor(src, key)
			if !ok {
				continue
			}
			fmt.Printf("%-28s %-10s %v\n", key, origin, src.Get(key))
			shown = true
		}

		if !shown {
			fmt.Println("(no settings)")
		}
		return nil
	},
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Remove a config key (reverts to default)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := newConfigSource()
		key := strings.ToLower(args[0])

		if _, err := validateKey(key); err != nil {
			return err
		}

		if !src.InFile(key) {
			return fmt.Errorf("key %q is not in config file", key)
		}

		src.Unset(key)

		if err := src.Save(); err != nil {
			return err
		}

		fmt.Printf("Removed %s from %s\n", key, src.FilePath())
		return nil
	},
}

var configOpenCmd = &cobra.Command{
	Use:   "open",
	Short: "Open config file in system editor",
	RunE: func(cmd *cobra.Command, args []string) error {
		src := newConfigSource()
		configPath := src.FilePath()

		if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// If the file doesn't exist yet, create it
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if err := src.Save(); err != nil {
				return fmt.Errorf("failed to create config file: %w", err)
			}
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			switch runtime.GOOS {
			case "windows":
				editor = "notepad"
			case "darwin":
				editor = "nano"
			default:
				editor = "nano"
				for _, e := range []string{"nvim", "vim", "micro", "vi", "nano"} {
					if _, err := exec.LookPath(e); err == nil {
						editor = e
						break
					}
				}
			}
		}

		editorCmd := exec.Command(editor, configPath)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr

		if err := editorCmd.Run(); err != nil {
			return fmt.Errorf("editor %q failed: %w", editor, err)
		}
		return nil
	},
}

var configAddCmd = &cobra.Command{
	Use:   "add <key> <value>",
	Short: "Append a value to a list config key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := newConfigSource()
		key := strings.ToLower(args[0])

		if _, err := validateKey(key); err != nil {
			return err
		}

		current := src.GetStringSlice(key)
		current = append(current, args[1])
		src.Set(key, current)

		if err := src.Save(); err != nil {
			return err
		}

		fmt.Printf("Added %q to %s in %s\n", args[1], key, src.FilePath())
		return nil
	},
}

var configRemoveCmd = &cobra.Command{
	Use:   "remove <key> <value>",
	Short: "Remove a value from a list config key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := newConfigSource()
		key := strings.ToLower(args[0])

		if _, err := validateKey(key); err != nil {
			return err
		}

		current := src.GetStringSlice(key)
		filtered := make([]string, 0, len(current))
		removed := false
		for _, item := range current {
			if item == args[1] {
				removed = true
			} else {
				filtered = append(filtered, item)
			}
		}

		if !removed {
			return fmt.Errorf("value %q not found in %s", args[1], key)
		}

		src.Set(key, filtered)

		if err := src.Save(); err != nil {
			return err
		}

		fmt.Printf("Removed %q from %s in %s\n", args[1], key, src.FilePath())
		return nil
	},
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Remove a config key (reverts to default)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := newViperForConfig()
		key := strings.ToLower(args[0])

		if _, err := validateKey(key); err != nil {
			return err
		}

		if !v.InConfig(key) {
			return fmt.Errorf("key %q is not in config file", key)
		}

		settings := v.AllSettings()
		delete(settings, key)

		// Rebuild the config with the key removed
		for k, val := range settings {
			v.Set(k, val)
		}

		configPath := getConfigPath(v)
		if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		if err := v.WriteConfigAs(configPath); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("Removed %s from %s\n", key, configPath)
		return nil
	},
}

var configOpenCmd = &cobra.Command{
	Use:   "open",
	Short: "Open config file in system editor",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := newViperForConfig()
		configPath := getConfigPath(v)

		if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// If the file doesn't exist yet, create it
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if err := v.WriteConfigAs(configPath); err != nil {
				return fmt.Errorf("failed to create config file: %w", err)
			}
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			switch runtime.GOOS {
			case "windows":
				editor = "notepad"
			case "darwin":
				editor = "nano"
			default:
				editor = "nano"
				for _, e := range []string{"nvim", "vim", "micro", "vi", "nano"} {
					if _, err := exec.LookPath(e); err == nil {
						editor = e
						break
					}
				}
			}
		}

		editorCmd := exec.Command(editor, configPath)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr

		if err := editorCmd.Run(); err != nil {
			return fmt.Errorf("editor %q failed: %w", editor, err)
		}
		return nil
	},
}

var configAddCmd = &cobra.Command{
	Use:   "add <key> <value>",
	Short: "Append a value to a list config key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := newViperForConfig()
		key := strings.ToLower(args[0])

		if _, err := validateKey(key); err != nil {
			return err
		}

		current := v.GetStringSlice(key)
		current = append(current, args[1])
		v.Set(key, current)

		configPath := getConfigPath(v)
		if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		if err := v.WriteConfigAs(configPath); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("Added %q to %s in %s\n", args[1], key, configPath)
		return nil
	},
}

var configRemoveCmd = &cobra.Command{
	Use:   "remove <key> <value>",
	Short: "Remove a value from a list config key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := newViperForConfig()
		key := strings.ToLower(args[0])

		if _, err := validateKey(key); err != nil {
			return err
		}

		current := v.GetStringSlice(key)
		filtered := make([]string, 0, len(current))
		removed := false
		for _, item := range current {
			if item == args[1] {
				removed = true
			} else {
				filtered = append(filtered, item)
			}
		}

		if !removed {
			return fmt.Errorf("value %q not found in %s", args[1], key)
		}

		v.Set(key, filtered)

		configPath := getConfigPath(v)
		if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		if err := v.WriteConfigAs(configPath); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("Removed %q from %s in %s\n", args[1], key, configPath)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		src := newConfigSource()
		path := src.FilePath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Println(path + " (file does not exist yet)")
		} else {
			fmt.Println(path)
		}
		return nil
	},
}

var configShellCmds = &cobra.Command{
	Use:   "shell-completions",
	Short: "Print shell completion setup instructions",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("To enable shell completion for config keys, add to your shell:")
		fmt.Println()
		fmt.Println("  # Bash (~/.bashrc)")
		fmt.Println(`  complete -W "` + strings.Join(knownKeyNames(), " ") + `" localgo`)
		fmt.Println()
		fmt.Println("  # Zsh (~/.zshrc)")
		fmt.Println(`  compadd -W "` + strings.Join(knownKeyNames(), " ") + `" -- ` + "${words[2]}")
		fmt.Println()
		fmt.Println("  # Or use: localgo completion bash/zsh/fish")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configUnsetCmd)
	configCmd.AddCommand(configOpenCmd)
	configCmd.AddCommand(configAddCmd)
	configCmd.AddCommand(configRemoveCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configShellCmds)

	configCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("config"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
	rootCmd.AddCommand(configCmd)
}
