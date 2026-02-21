package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"

	"github.com/fsnotify/fsnotify"
)

type Config struct {
	NginxAccessLogPath string
	NginxErrorLogPath  string
	SSHAuthLogPath     string
	Port               int
	GeoIPCityPath      string // Path to GeoLite2-City.mmdb
	GeoIPASNPath       string // Path to GeoLite2-ASN.mmdb
	WebhookURL         string // Optional URL for critical alerts e.g. Slack/Discord
	RulesPath          string // Path to custom rules (e.g., process blacklist)
}

func Load() *Config {
	return &Config{
		NginxAccessLogPath: getEnv("NGINX_ACCESS_LOG_PATH", "/var/log/nginx/access.log"),
		NginxErrorLogPath:  getEnv("NGINX_ERROR_LOG_PATH", "/var/log/nginx/error.log"),
		SSHAuthLogPath:     getEnv("SSH_AUTH_LOG_PATH", "/var/log/auth.log"),
		Port:               getEnvInt("PORT", 9102),
		GeoIPCityPath:      getEnv("GEOIP_CITY_PATH", ""), // Empty means disabled
		GeoIPASNPath:       getEnv("GEOIP_ASN_PATH", ""),
		WebhookURL:         getEnv("WEBHOOK_URL", ""),
		RulesPath:          getEnv("RULES_PATH", "rules.json"), // Default simple JSON for dynamic rules
	}
}

type Rules struct {
	ProcessBlacklist []string `json:"ProcessBlacklist"`
}

// LoadRules reads the rules file
func (c *Config) LoadRules() (*Rules, error) {
	if c.RulesPath == "" {
		return nil, os.ErrNotExist
	}
	
	data, err := os.ReadFile(c.RulesPath)
	if err != nil {
		return nil, err
	}
	
	var r Rules
	err = json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}
	
	return &r, nil
}

// WatchConfig watches the rules file for changes and triggers the callback
func (c *Config) WatchConfig(callback func()) error {
	if c.RulesPath == "" {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Printf("Rules file %s modified. Reloading...", event.Name)
					callback()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	err = watcher.Add(c.RulesPath)
	if err != nil {
		// Log but don't fail completely if rule file doesn't exist yet
		log.Printf("Could not watch rules file: %v. Creating empty one.", err)
		os.WriteFile(c.RulesPath, []byte(`{"ProcessBlacklist": ["nc", "nmap", "hydra", "john", "xmrig"]}`), 0644)
		watcher.Add(c.RulesPath)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}
