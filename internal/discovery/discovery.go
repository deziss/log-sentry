package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DetectedService struct {
	Name    string
	PID     int
	LogPath string // Best guess or empty
}

type AutoDiscover struct {
	ProcRoot string
}

func NewAutoDiscover() *AutoDiscover {
	return &AutoDiscover{
		ProcRoot: "/proc",
	}
}

// targetProcesses defines what we are looking for
var targetProcesses = map[string]*regexp.Regexp{
	"nginx":  regexp.MustCompile(`^nginx`),
	"apache": regexp.MustCompile(`^(apache2|httpd)`),
	"caddy":  regexp.MustCompile(`^caddy`),
	"tomcat": regexp.MustCompile(`^java`), // Broad, need to check cmdline for 'catalina'
	"traefik": regexp.MustCompile(`^traefik`),
	"haproxy": regexp.MustCompile(`^haproxy`),
	"envoy":   regexp.MustCompile(`^envoy`),
	"lighttpd": regexp.MustCompile(`^lighttpd`),
}

func (ad *AutoDiscover) Scan() ([]DetectedService, error) {
	entries, err := os.ReadDir(ad.ProcRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read proc root: %v", err)
	}

	var services []DetectedService

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		// check if name is a PID
		pid := entry.Name()
		if _, err := fmt.Sscanf(pid, "%d", new(int)); err != nil {
			continue
		}

		// Read comm
		commPath := filepath.Join(ad.ProcRoot, pid, "comm")
		commBytes, err := os.ReadFile(commPath)
		if err != nil {
			continue 
		}
		comm := strings.TrimSpace(string(commBytes))

		// Check against targets
		for serviceName, regex := range targetProcesses {
			if regex.MatchString(comm) {
				// Special check for Tomcat/Java
				if serviceName == "tomcat" {
					if !ad.isTomcat(pid) {
						continue
					}
				}

				services = append(services, DetectedService{
					Name:    serviceName,
					PID:     toPID(pid),
					LogPath: ad.guessLogPath(serviceName),
				})
			}
		}
	}
	return services, nil
}

func (ad *AutoDiscover) isTomcat(pid string) bool {
	cmdlinePath := filepath.Join(ad.ProcRoot, pid, "cmdline")
	cmdlineBytes, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return false
	}
	// cmdline arguments are null separated
	cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
	return strings.Contains(cmdline, "catalina") || strings.Contains(cmdline, "tomcat")
}

func (ad *AutoDiscover) guessLogPath(service string) string {
	// These are defaults, in a real scenario we might parse the config file 
	// found via /proc/[pid]/cwd or cmdline args
	switch service {
	case "nginx":
		return "/var/log/nginx/access.log"
	case "apache":
		return "/var/log/apache2/access.log" // Debian default
	case "caddy":
		return "/var/log/caddy/access.log"
	case "tomcat":
		return "/usr/local/tomcat/logs/localhost_access_log.txt"
	case "traefik":
		return "/var/log/traefik/access.log" // Common default
	case "haproxy":
		return "/var/log/haproxy.log"
	case "envoy":
		return "/var/log/envoy/access.log"
	case "lighttpd":
		return "/var/log/lighttpd/access.log"
	}
	return ""
}

func toPID(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}
