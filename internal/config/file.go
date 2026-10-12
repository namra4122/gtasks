package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/BRO3886/gtasks/internal/utils"
)

// k is the package-level koanf instance loaded by LoadAppConfig.
var k = koanf.New(".")

// LoadAppConfig loads configuration using the following priority order (highest first):
//
//  1. Environment variables (GTASKS_* prefix)
//  2. Config file: config.toml / config.yaml / config.json in the config directory
//
// A missing config file is silently ignored.
// A malformed config file logs a warning and falls through to env vars.
func LoadAppConfig() {
	k = koanf.New(".") // reset so repeated calls don't accumulate state
	cfgDir := GetInstallLocation()

	// 3. Config file (lowest priority — loaded first, overridden by layers above)
	for _, candidate := range []struct {
		name   string
		parser koanf.Parser
	}{
		{"config.toml", toml.Parser()},
		{"config.yaml", yaml.Parser()},
		{"config.yml", yaml.Parser()},
		{"config.json", json.Parser()},
	} {
		cfgPath := filepath.Join(cfgDir, candidate.name)
		if _, err := os.Stat(cfgPath); err == nil {
			if err := k.Load(file.Provider(cfgPath), candidate.parser); err != nil {
				utils.Warn("Could not parse config file %s: %v\n", cfgPath, err)
			}
			break // use the first one found
		}
	}

	// 1. Environment variables — GTASKS_ prefix, mapped to dotted keys
	// e.g. GTASKS_CLIENT_ID -> credentials.client_id
	//      GTASKS_DEFAULT_TASKLIST -> tasks.default_task_list
	//      GTASKS_DEFAULT_CALENDAR -> calendar.default_calendar
	k.Load(env.Provider("GTASKS_", ".", func(s string) string {
		s = strings.TrimPrefix(s, "GTASKS_")
		s = strings.ToLower(s)
		switch s {
		case "client_id":
			return "credentials.client_id"
		case "client_secret":
			return "credentials.client_secret"
		case "default_tasklist":
			return "tasks.default_task_list"
		case "default_calendar":
			return "calendar.default_calendar"
		}
		return "" // skip unrecognized GTASKS_* vars
	}), nil)
}

// GetDefaultTaskList returns the default task list from config/env, or empty string.
func GetDefaultTaskList() string {
	return k.String("tasks.default_task_list")
}

// GetDefaultCalendar returns the default calendar from config/env, or "primary".
func GetDefaultCalendar() string {
	if c := k.String("calendar.default_calendar"); c != "" {
		return c
	}
	return "primary"
}

// GetCredentials returns client ID and secret from config/env.
func GetCredentials() (clientID, clientSecret string) {
	return k.String("credentials.client_id"), k.String("credentials.client_secret")
}
