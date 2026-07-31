package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Source is a lightweight replacement for viper. It resolves config values
// with the precedence: programmatic overrides > environment > file > defaults.
//
// Environment variables are named LOCALSEND_<KEY> with '-' replaced by '_'.
type Source struct {
	filePath  string
	file      map[string]any
	overrides map[string]any
	defaults  map[string]any
}

var sourceDefaults = map[string]any{
	"port":            DefaultPort,
	"multicast_group": DefaultMulticastGroup,
	"concurrency":     4,
}

func newSource() *Source {
	return &Source{
		file:      make(map[string]any),
		overrides: make(map[string]any),
		defaults:  sourceDefaults,
	}
}

// LoadSource searches the standard config directories for a config file and
// returns a Source bound to it. Returns an empty Source if no file is found.
func LoadSource() *Source {
	s := newSource()
	for _, dir := range []string{
		os.ExpandEnv("$HOME/.config/localgo"),
		os.ExpandEnv("$HOME/.local/etc/localgo"),
		".",
	} {
		for _, name := range []string{"config.yaml", "config.yml"} {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err == nil {
				s.filePath = p
				s.readFile()
				return s
			}
		}
	}
	return s
}

// LoadSourceFile returns a Source bound to an explicitly selected config file.
func LoadSourceFile(path string) (*Source, error) {
	s := newSource()
	s.filePath = path
	if _, err := os.Stat(path); err != nil {
		return s, err
	}
	s.readFile()
	return s, nil
}

// NewSourceFromMap returns a Source initialised with the given programmatic
// overrides (used for tests and callers that supply values directly).
func NewSourceFromMap(m map[string]any) *Source {
	s := newSource()
	for k, v := range m {
		s.overrides[k] = v
	}
	return s
}

// SetConfigFile overrides the config file path used by FilePath and Save.
func (s *Source) SetConfigFile(path string) {
	s.filePath = path
}

func (s *Source) readFile() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	if err := yaml.Unmarshal(data, &s.file); err != nil {
		s.file = make(map[string]any)
	}
	if s.file == nil {
		s.file = make(map[string]any)
	}
}

func envName(key string) string {
	return "LOCALSEND_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
}

// raw resolves a key across all sources without any type coercion.
func (s *Source) raw(key string) (any, bool) {
	if v, ok := s.overrides[key]; ok {
		return v, true
	}
	if v, ok := os.LookupEnv(envName(key)); ok {
		return v, true
	}
	if v, ok := s.file[key]; ok {
		return v, true
	}
	if v, ok := s.defaults[key]; ok {
		return v, true
	}
	return nil, false
}

// IsSet reports whether the key has a value from any source.
func (s *Source) IsSet(key string) bool {
	_, ok := s.raw(key)
	return ok
}

// InFile reports whether the key is present in the config file.
func (s *Source) InFile(key string) bool {
	_, ok := s.file[key]
	return ok
}

// FilePath returns the resolved config file path, or the default location
// when no file has been found or selected.
func (s *Source) FilePath() string {
	if s.filePath != "" {
		return s.filePath
	}
	return os.ExpandEnv("$HOME/.config/localgo/config.yaml")
}

// Set stores a programmatic override for the key.
func (s *Source) Set(key string, val any) {
	s.overrides[key] = val
}

// SetDefault registers a fallback value used when no other source has the key.
func (s *Source) SetDefault(key string, val any) {
	s.defaults[key] = val
}

// Unset removes the key from the file and overrides.
func (s *Source) Unset(key string) {
	delete(s.file, key)
	delete(s.overrides, key)
}

// Save writes the file entries merged with programmatic overrides to the
// resolved config path, creating parent directories as needed.
func (s *Source) Save() error {
	for k, v := range s.overrides {
		s.file[k] = v
	}
	s.overrides = make(map[string]any)

	path := s.FilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(s.file)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// GetString returns the string form of the resolved value.
func (s *Source) GetString(key string) string {
	v, ok := s.raw(key)
	if !ok {
		return ""
	}
	return toString(v)
}

// GetInt returns the int form of the resolved value.
func (s *Source) GetInt(key string) int {
	v, ok := s.raw(key)
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(t); err == nil {
			return n
		}
	case bool:
		if t {
			return 1
		}
	}
	return 0
}

// GetStringSlice returns the resolved value as a list of strings. A
// comma-separated environment variable or a YAML scalar yields a single item.
func (s *Source) GetStringSlice(key string) []string {
	v, ok := s.raw(key)
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, toString(item))
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	default:
		return []string{toString(t)}
	}
}

// Get returns the resolved value as-is.
func (s *Source) Get(key string) any {
	v, ok := s.raw(key)
	if !ok {
		return nil
	}
	return v
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []string:
		return strings.Join(t, ",")
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, toString(item))
		}
		return strings.Join(parts, ",")
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}
