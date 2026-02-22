package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"

	"github.com/fsnotify/fsnotify"
)

type Config struct {
	Port               int
	GeoIPCityPath      string // Path to GeoLite2-City.mmdb
	GeoIPASNPath       string // Path to GeoLite2-ASN.mmdb
	WebhookURL         string // Optional URL for critical alerts e.g. Slack/Discord
	RulesPath          string // Path to custom rules (e.g., process blacklist)
	ServicesConfigPath string // Path to services.json
	Services           []ServiceDef

	// Resource Recorder (threshold-based crash detection)
	SnapshotInterval int     // seconds between polls (default: 5)
	Threshold        float64 // percentage to trigger recording (default: 90)
	LokiURL          string  // Loki push URL (e.g. http://loki:3100)

	// Storage
	DBPath        string // path to BoltDB file (default: "data/logsentry.db")
	RetentionDays int    // days to keep crash/attack data (default: 30)
}

// ServiceDef defines a single log source to monitor.
// Configured entirely via services.json — no code changes needed.
type ServiceDef struct {
	Name    string `json:"name"`     // Human-readable label e.g. "web-1"
	Type    string `json:"type"`     // Parser type: "nginx", "apache", "ssh", etc.
	LogPath string `json:"log_path"` // Absolute path to log file
	Enabled bool   `json:"enabled"`  // Toggle on/off
}

type servicesFile struct {
	Services []ServiceDef `json:"services"`
}

func Load() *Config {
	cfg := &Config{
		Port:               getEnvInt("PORT", 9102),
		GeoIPCityPath:      getEnv("GEOIP_CITY_PATH", ""),
		GeoIPASNPath:       getEnv("GEOIP_ASN_PATH", ""),
		WebhookURL:         getEnv("WEBHOOK_URL", ""),
		RulesPath:          getEnv("RULES_PATH", "rules.json"),
		ServicesConfigPath: getEnv("SERVICES_CONFIG", "services.json"),
		SnapshotInterval:   getEnvInt("SNAPSHOT_INTERVAL", 5),
		Threshold:          float64(getEnvInt("THRESHOLD", 90)),
		LokiURL:            getEnv("LOKI_URL", ""),
		DBPath:             getEnv("DB_PATH", "data/logsentry.db"),
		RetentionDays:      getEnvInt("RETENTION_DAYS", 30),
	}

	// Load services from JSON
	if err := cfg.LoadServices(); err != nil {
		log.Printf("Could not load services config: %v. Using defaults.", err)
		// Fallback defaults so it still works out of the box
		cfg.Services = []ServiceDef{
			{Name: "nginx", Type: "nginx", LogPath: getEnv("NGINX_ACCESS_LOG_PATH", "/var/log/nginx/access.log"), Enabled: true},
			{Name: "ssh", Type: "ssh", LogPath: getEnv("SSH_AUTH_LOG_PATH", "/var/log/auth.log"), Enabled: true},
		}
	}
	return cfg
}

// LoadServices reads the services config file
func (c *Config) LoadServices() error {
	data, err := os.ReadFile(c.ServicesConfigPath)
	if err != nil {
		return err
	}

	var sf servicesFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return err
	}

	c.Services = sf.Services
	log.Printf("Loaded %d service definitions from %s", len(c.Services), c.ServicesConfigPath)
	return nil
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
