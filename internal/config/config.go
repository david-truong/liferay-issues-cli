package config

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdx/go-netrc"
	"github.com/spf13/viper"
)

var Version = "dev"

type Config struct {
	Jira     JiraConfig     `mapstructure:"jira"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Defaults DefaultsConfig `mapstructure:"defaults"`
}

type JiraConfig struct {
	Instance         string `mapstructure:"instance"`
	DefaultProject   string `mapstructure:"default_project"`
	DefaultBoard     int    `mapstructure:"default_board"`
	DefaultComponent string `mapstructure:"default_component"`
}

type AuthConfig struct {
	Email string `mapstructure:"email"`
	Token string `mapstructure:"token"`
}

type DefaultsConfig struct {
	IssueType string `mapstructure:"issue_type"`
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "issues")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "issues")
}

func ConfigFilePath() string {
	return filepath.Join(configDir(), "config.yaml")
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir())

	// Defaults
	viper.SetDefault("jira.instance", "liferay.atlassian.net")
	viper.SetDefault("jira.default_project", "")
	viper.SetDefault("defaults.issue_type", "Task")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

func EnsureConfigDir() error {
	return os.MkdirAll(configDir(), 0755)
}

// ResolveAuth returns an http.Header with authorization, checking:
// 1. Environment variables (JIRA_API_TOKEN + JIRA_USER)
// 2. Config file (auth.email + auth.token)
// 3. .netrc
func ResolveAuth(cfg *Config) (http.Header, error) {
	// 1. Env vars
	token := os.Getenv("JIRA_API_TOKEN")
	user := os.Getenv("JIRA_USER")
	if token != "" && user != "" {
		return basicAuthHeader(user, token), nil
	}

	// 2. Config file
	if cfg.Auth.Email != "" && cfg.Auth.Token != "" {
		return basicAuthHeader(cfg.Auth.Email, cfg.Auth.Token), nil
	}

	// 3. .netrc
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}

	netrcPath := filepath.Join(home, ".netrc")
	rc, err := netrc.Parse(netrcPath)
	if err != nil {
		return nil, fmt.Errorf("no credentials found.\n\nConfigure with one of:\n  1. issues config set auth.email you@company.com\n     issues config set auth.token YOUR_API_TOKEN\n  2. export JIRA_API_TOKEN=... and JIRA_USER=...\n  3. Add machine %s to ~/.netrc", cfg.Jira.Instance)
	}

	machine := rc.Machine(cfg.Jira.Instance)
	if machine == nil {
		// Try with https:// prefix variations
		for _, m := range rc.Machines() {
			if strings.Contains(m.Name, cfg.Jira.Instance) {
				machine = m
				break
			}
		}
	}

	if machine == nil {
		return nil, fmt.Errorf("no entry for %s in ~/.netrc.\n\nConfigure with one of:\n  1. issues config set auth.email you@company.com\n     issues config set auth.token YOUR_API_TOKEN\n  2. export JIRA_API_TOKEN=... and JIRA_USER=...\n  3. Add machine %s to ~/.netrc", cfg.Jira.Instance, cfg.Jira.Instance)
	}

	return basicAuthHeader(machine.Get("login"), machine.Get("password")), nil
}

func basicAuthHeader(user, token string) http.Header {
	h := http.Header{}
	cred := base64.StdEncoding.EncodeToString([]byte(user + ":" + token))
	h.Set("Authorization", "Basic "+cred)
	return h
}
