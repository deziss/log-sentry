package config

import (
	"os"
	"strconv"
)

type Config struct {
	NginxAccessLogPath string
	NginxErrorLogPath  string
	SSHAuthLogPath     string
	Port               int
	EnableMagicLogAccess bool
}

func Load() *Config {
	return &Config{
		NginxAccessLogPath: getEnv("NGINX_ACCESS_LOG_PATH", "/var/log/nginx/access.log"),
		NginxErrorLogPath:  getEnv("NGINX_ERROR_LOG_PATH", "/var/log/nginx/error.log"),
		SSHAuthLogPath:     getEnv("SSH_AUTH_LOG_PATH", "/var/log/auth.log"),
		Port:               getEnvInt("PORT", 9102),
		EnableMagicLogAccess: getEnvBool("ENABLE_MAGIC_LOG_ACCESS", false),
	}
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

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return fallback
}
