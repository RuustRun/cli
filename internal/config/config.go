// Package config loads and saves the ruust CLI configuration.
//
// The config lives at ~/.config/ruust/config.json and holds the API host, the
// current session token, and the signed-in email. Environment variables
// RUUST_HOST and RUUST_TOKEN take precedence over the stored values so scripts
// and CI can override them without touching the file on disk.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// DefaultHost is the API host used when none is configured. It points at the hosted
// control plane, so a freshly installed CLI talks to production out of the box. For
// local development, override it with RUUST_HOST=http://localhost:3939 (or `ruust
// login` against a local control plane, which stores the host in the config file).
const DefaultHost = "https://ruust.run"

// Config is the on-disk CLI configuration.
type Config struct {
	Host  string `json:"host"`
	Token string `json:"token"`
	Email string `json:"email"`
}

// configDir returns ~/.config/ruust, honouring XDG_CONFIG_HOME when set.
func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ruust"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ruust"), nil
}

// configPath returns the full path to config.json.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config from disk. A missing file is not an error: it returns a
// config populated with defaults so first-run behaves sensibly.
func Load() (*Config, error) {
	c := &Config{Host: DefaultHost}

	path, err := configPath()
	if err != nil {
		return c, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}
		return c, err
	}

	if err := json.Unmarshal(data, c); err != nil {
		return c, err
	}
	if c.Host == "" {
		c.Host = DefaultHost
	}
	return c, nil
}

// Save writes the config to disk, creating the directory if needed. The file is
// written with owner-only permissions because it holds a session token.
func (c *Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	path := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ResolveHost returns the effective host for a config, applying the RUUST_HOST
// environment override which takes precedence over the stored value.
func (c *Config) ResolveHost() string {
	return Host(c)
}

// ResolveToken returns the effective token for a config, applying the
// RUUST_TOKEN environment override which takes precedence over the stored value.
func (c *Config) ResolveToken() string {
	return Token(c)
}

// Host returns the effective host, applying the RUUST_HOST override. It is safe
// to pass a nil config, in which case the default host (or the override) is used.
func Host(c *Config) string {
	if env := os.Getenv("RUUST_HOST"); env != "" {
		return env
	}
	if c != nil && c.Host != "" {
		return c.Host
	}
	return DefaultHost
}

// Token returns the effective token, applying the RUUST_TOKEN override. It is
// safe to pass a nil config, in which case only the override (if any) is used.
func Token(c *Config) string {
	if env := os.Getenv("RUUST_TOKEN"); env != "" {
		return env
	}
	if c != nil {
		return c.Token
	}
	return ""
}
