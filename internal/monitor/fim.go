package monitor

import (
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type FIM struct {
	Paths []string
	FileHashes map[string]int64 // For V2.2 we monitor ModTime and Size for speed/simplicity
	ChangeMetric *prometheus.CounterVec
}

func NewFIM() *FIM {
	return &FIM{
		Paths: []string{},
		FileHashes: make(map[string]int64),
		ChangeMetric: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sensitive_file_changed_total",
			Help: "Total number of detected changes to sensitive files",
		}, []string{"path", "severity"}),
	}
}

func (f *FIM) Register(reg prometheus.Registerer) {
	reg.MustRegister(f.ChangeMetric)
}

func (f *FIM) AddPath(path string) {
	f.Paths = append(f.Paths, path)
	// Seed initial state
	info, err := os.Stat(path)
	if err == nil {
		f.FileHashes[path] = info.ModTime().UnixNano()
	}
}

func (f *FIM) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			f.checkAll()
			<-ticker.C
		}
	}()
}

func (f *FIM) checkAll() {
	for _, path := range f.Paths {
		info, err := os.Stat(path)
		if err != nil {
			// File gone?
			continue
		}
		
		current := info.ModTime().UnixNano()
		last, exists := f.FileHashes[path]
		
		if exists && current != last {
			f.ChangeMetric.WithLabelValues(path, "critical").Inc()
			f.FileHashes[path] = current
		} else if !exists {
			f.FileHashes[path] = current
		}
	}
}
