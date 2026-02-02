package syslog

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"

	"log-sentry/internal/analyzer"
	"log-sentry/internal/collector"
	"log-sentry/internal/enricher"
	"log-sentry/internal/parser"
)

type SyslogServer struct {
	Port      int
	Collector *collector.LogCollector
	Analyzer  *analyzer.Analyzer
	Enricher  *enricher.Enricher
	// Mappers could allow routing based on syslog tags to specific parsers
	// For simplicity V2.1: We assume incoming syslog is likely HAProxy or generic web logs
}

func NewSyslogServer(port int, coll *collector.LogCollector, a *analyzer.Analyzer, e *enricher.Enricher) *SyslogServer {
	return &SyslogServer{
		Port:      port,
		Collector: coll,
		Analyzer:  a,
		Enricher:  e,
	}
}

func (s *SyslogServer) Start() {
	// Start UDP Listener
	go s.startUDP()
	// Start TCP Listener
	go s.startTCP()
}

func (s *SyslogServer) startUDP() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.Port)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Printf("Syslog UDP listen error: %v", err)
		return
	}
	defer conn.Close()
	log.Printf("Syslog UDP listening on %s", addr)

	buf := make([]byte, 4096)
	haproxyParser := &parser.HAProxyParser{} // Default handler for now

	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("Syslog UDP read error: %v", err)
			continue
		}
		line := strings.TrimSpace(string(buf[:n]))
		s.processLine(line, haproxyParser)
	}
}

func (s *SyslogServer) startTCP() {
	addr := fmt.Sprintf("0.0.0.0:%d", s.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("Syslog TCP listen error: %v", err)
		return
	}
	defer ln.Close()
	log.Printf("Syslog TCP listening on %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go s.handleTCPConn(conn)
	}
}

func (s *SyslogServer) handleTCPConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	haproxyParser := &parser.HAProxyParser{}

	for scanner.Scan() {
		s.processLine(scanner.Text(), haproxyParser)
	}
}

func (s *SyslogServer) processLine(line string, p parser.LogParser) {
	// In a real generic syslog server, we would parse the PRI/Header to determine the app.
	// Here we try to parse with HAProxy parser first, or fallback.
	entry, err := p.Parse(line)
	if err != nil {
		// If failed, maybe debug log or try another parser?
		return
	}
	
	entry.Service = "syslog_ingest" // or entryFromParser if available
	
	attack := s.Analyzer.DetectAttack(entry.Path, entry.UserAgent)
	// Syslog doesn't use AnomalyDetector yet (or pass it in?)
	// For now, pass empty string constant
	netType := s.Enricher.ClassifyIP(entry.RemoteIP)
	s.Collector.ProcessWeb(entry, attack, "", netType)
}
