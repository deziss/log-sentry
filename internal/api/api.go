package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"log-sentry/internal/config"
	"log-sentry/internal/parser"
	"log-sentry/internal/recorder"
)

// API holds shared state for all handlers
type API struct {
	Config   *config.Config
	Recorder *recorder.ResourceRecorder
	mu       sync.RWMutex
}

func NewAPI(cfg *config.Config, rec *recorder.ResourceRecorder) *API {
	return &API{Config: cfg, Recorder: rec}
}

// RegisterRoutes mounts all API endpoints on the given mux
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/services", a.cors(a.handleServices))
	mux.HandleFunc("/api/rules", a.cors(a.handleRules))
	mux.HandleFunc("/api/parsers", a.cors(a.handleParsers))
	mux.HandleFunc("/api/health", a.cors(a.handleHealth))
	mux.HandleFunc("/api/forensic", a.cors(a.handleForensic))
	mux.HandleFunc("/api/snapshots", a.cors(a.handleSnapshots))
}

// ── CORS middleware ──────────────────────────────────────────────
func (a *API) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// ── Services CRUD ────────────────────────────────────────────────

func (a *API) handleServices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getServices(w, r)
	case http.MethodPost:
		a.addService(w, r)
	case http.MethodPut:
		a.updateService(w, r)
	case http.MethodDelete:
		a.deleteService(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) getServices(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	writeJSON(w, http.StatusOK, a.Config.Services)
}

func (a *API) addService(w http.ResponseWriter, r *http.Request) {
	var svc config.ServiceDef
	if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	a.Config.Services = append(a.Config.Services, svc)
	a.mu.Unlock()

	if err := a.persistServices(); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, svc)
}

func (a *API) updateService(w http.ResponseWriter, r *http.Request) {
	// Name passed as query param: /api/services?name=web-1
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing ?name= parameter", http.StatusBadRequest)
		return
	}

	var updated config.ServiceDef
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	found := false
	for i, svc := range a.Config.Services {
		if svc.Name == name {
			a.Config.Services[i] = updated
			found = true
			break
		}
	}
	a.mu.Unlock()

	if !found {
		http.Error(w, "Service not found: "+name, http.StatusNotFound)
		return
	}

	if err := a.persistServices(); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteService(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing ?name= parameter", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	found := false
	for i, svc := range a.Config.Services {
		if svc.Name == name {
			a.Config.Services = append(a.Config.Services[:i], a.Config.Services[i+1:]...)
			found = true
			break
		}
	}
	a.mu.Unlock()

	if !found {
		http.Error(w, "Service not found: "+name, http.StatusNotFound)
		return
	}

	if err := a.persistServices(); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"deleted": name})
}

func (a *API) persistServices() error {
	a.mu.RLock()
	data, err := json.MarshalIndent(struct {
		Services []config.ServiceDef `json:"services"`
	}{Services: a.Config.Services}, "", "  ")
	a.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(a.Config.ServicesConfigPath, data, 0644)
}

// ── Rules CRUD ───────────────────────────────────────────────────

func (a *API) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getRules(w, r)
	case http.MethodPut:
		a.updateRules(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) getRules(w http.ResponseWriter, _ *http.Request) {
	rules, err := a.Config.LoadRules()
	if err != nil {
		// Return empty defaults if file doesn't exist
		writeJSON(w, http.StatusOK, config.Rules{
			ProcessBlacklist: []string{"nc", "nmap", "hydra", "john", "xmrig"},
		})
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (a *API) updateRules(w http.ResponseWriter, r *http.Request) {
	var rules config.Rules
	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(a.Config.RulesPath, data, 0644); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// fsnotify will pick this up and trigger hot-reload
	log.Printf("Rules updated via API")
	writeJSON(w, http.StatusOK, rules)
}

// ── Parsers ──────────────────────────────────────────────────────

func (a *API) handleParsers(w http.ResponseWriter, _ *http.Request) {
	// Include "ssh" as a virtual type for the UI
	available := parser.AvailableParsers()
	if !contains(available, "ssh") {
		available = append(available, "ssh")
	}
	writeJSON(w, http.StatusOK, available)
}

// ── Health ───────────────────────────────────────────────────────

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	serviceCount := len(a.Config.Services)
	a.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"services": serviceCount,
		"parsers":  len(parser.AvailableParsers()),
	})
}

// ── Helpers ──────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if strings.EqualFold(a, e) {
			return true
		}
	}
	return false
}

// ── Forensic Analysis ────────────────────────────────────────────

func (a *API) handleForensic(w http.ResponseWriter, _ *http.Request) {
	snaps := a.Recorder.GetSnapshots(0) // all snapshots
	report := recorder.Analyze(snaps)
	writeJSON(w, http.StatusOK, report)
}

func (a *API) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	n := 60 // default: last 60 snapshots (5 min at 5s intervals)
	if q := r.URL.Query().Get("last"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	snaps := a.Recorder.GetSnapshots(n)
	writeJSON(w, http.StatusOK, snaps)
}
