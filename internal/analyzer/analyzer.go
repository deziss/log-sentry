package analyzer

import (
	"encoding/base64"
	"net/url"
	"regexp"
)

type AttackType string

const (
	SQLInjection  AttackType = "sqli"
	XSS           AttackType = "xss"
	PathTraversal AttackType = "path_traversal"
	Scanner       AttackType = "scanner"
	None          AttackType = "none"
)

type Analyzer struct {
	sqliRegex    *regexp.Regexp
	xssRegex     *regexp.Regexp
	pathTravRegex *regexp.Regexp
	scannerRegex *regexp.Regexp
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		// Basic SQLi patterns: UNION SELECT, OR 1=1, --, etc.
		sqliRegex: regexp.MustCompile(`(?i)(union\s+select|or\s+1=1|\s+or\s+true|--|;\s*drop\s+table)`),
		
		// Basic XSS patterns: <script>, javascript:, on(event)=
		xssRegex: regexp.MustCompile(`(?i)(<script|javascript:|on\w+=|alert\()`),

		// Path Traversal: ../..
		pathTravRegex: regexp.MustCompile(`\.\./\.\.`),

		// Common Scanners User-Agents (simplified)
		scannerRegex: regexp.MustCompile(`(?i)(nessus|nmap|nikto|sqlmap|burp)`),
	}
}

type AttackResult struct {
	Detected bool
	Type     string
	Severity string // critical, high, medium, low
}

// DetectAttack analyzes the request path, query params (if in path), and user agent
func (a *Analyzer) DetectAttack(path, userAgent string) AttackResult {
	// 1. Decode URL path to catch %20, %27, etc
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		decodedPath = path // Fallback to raw if decode fails
	}
	
	// 2. Attempt Base64 decoding (many attackers base64 encode payloads like b3IgMT0x = or 1=1)
	// We extract potential base64 strings and decode them (simplified: try decoding whole string)
	var b64Decoded string
	if b, err := base64.StdEncoding.DecodeString(path); err == nil {
		b64Decoded = string(b)
	}

	targets := []string{path, decodedPath}
	if b64Decoded != "" {
		targets = append(targets, b64Decoded)
	}

	for _, target := range targets {
		if a.sqliRegex.MatchString(target) {
			return AttackResult{Detected: true, Type: "SQL Injection", Severity: "critical"}
		}
		if a.xssRegex.MatchString(target) {
			return AttackResult{Detected: true, Type: "XSS", Severity: "high"}
		}
		if a.pathTravRegex.MatchString(target) {
			return AttackResult{Detected: true, Type: "Path Traversal", Severity: "high"}
		}
	}
	
	// User-agent checks usually don't need intense decoding, but could be added if needed
	if a.scannerRegex.MatchString(userAgent) {
		return AttackResult{Detected: true, Type: "Scanner", Severity: "medium"}
	}
	
	return AttackResult{Detected: false}
}

// CheckDataExfiltration checks for unusually large response bodies
func (a *Analyzer) CheckDataExfiltration(bytesSent int) AttackResult {
	const OneHundredMB = 100 * 1024 * 1024
	if bytesSent > OneHundredMB {
		return AttackResult{Detected: true, Type: "Data Exfiltration (Large Download)", Severity: "high"}
	}
	return AttackResult{Detected: false}
}
