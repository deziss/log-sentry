const API_BASE = '/api';

export interface ServiceDef {
  name: string;
  type: string;
  log_path: string;
  enabled: boolean;
}

export interface Rules {
  ProcessBlacklist: string[];
}

export interface HealthStatus {
  status: string;
  services: number;
  parsers: number;
}

export interface AppConfig {
  loki_url: string;
  prometheus_url: string;
  grafana_url: string;
}

// ── Services ─────────────────────────────────────────────────────

export async function fetchServices(): Promise<ServiceDef[]> {
  const res = await fetch(`${API_BASE}/services`);
  if (!res.ok) throw new Error('Failed to fetch services');
  return res.json();
}

export async function addService(svc: ServiceDef): Promise<ServiceDef> {
  const res = await fetch(`${API_BASE}/services`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(svc),
  });
  if (!res.ok) throw new Error('Failed to add service');
  return res.json();
}

export async function updateService(name: string, svc: ServiceDef): Promise<ServiceDef> {
  const res = await fetch(`${API_BASE}/services?name=${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(svc),
  });
  if (!res.ok) throw new Error('Failed to update service');
  return res.json();
}

export async function toggleService(name: string, enabled: boolean): Promise<ServiceDef> {
  const services = await fetchServices();
  const svc = services.find(s => s.name === name);
  if (!svc) throw new Error(`Service "${name}" not found`);
  return updateService(name, { ...svc, enabled });
}

export async function deleteService(name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/services?name=${encodeURIComponent(name)}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete service');
}

// ── Rules ────────────────────────────────────────────────────────

export async function fetchRules(): Promise<Rules> {
  const res = await fetch(`${API_BASE}/rules`);
  if (!res.ok) throw new Error('Failed to fetch rules');
  return res.json();
}

export async function updateRules(rules: Rules): Promise<Rules> {
  const res = await fetch(`${API_BASE}/rules`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(rules),
  });
  if (!res.ok) throw new Error('Failed to update rules');
  return res.json();
}

// ── Parsers ──────────────────────────────────────────────────────

export async function fetchParsers(): Promise<string[]> {
  const res = await fetch(`${API_BASE}/parsers`);
  if (!res.ok) throw new Error('Failed to fetch parsers');
  return res.json();
}

// ── Health ───────────────────────────────────────────────────────

export async function fetchHealth(): Promise<HealthStatus> {
  const res = await fetch(`${API_BASE}/health`);
  if (!res.ok) throw new Error('Failed to fetch health');
  return res.json();
}

// ── Config ──────────────────────────────────────────────────────

export async function fetchConfig(): Promise<AppConfig> {
  const res = await fetch(`${API_BASE}/config`);
  if (!res.ok) throw new Error('Failed to fetch config');
  return res.json();
}

// ── Paginated Types ─────────────────────────────────────────────

export interface PaginatedResult<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface CrashSummary {
  id: string;
  started_at: string;
  ended_at: string;
  trigger: string;
  verdict: string;
  severity: string;
  resolved: boolean;
  snapshot_count: number;
}

export interface AttackEntry {
  id: string;
  timestamp: string;
  service: string;
  type: string;
  severity: string;
  source_ip: string;
  endpoint: string;
  country?: string;
  asn?: string;
  network_type?: string;
  details?: string;
}

export interface AggregatedStats {
  total_crashes: number;
  active_crashes: number;
  avg_duration_sec: number;
  total_attacks: number;
  top_attack_type: string;
  top_attacked_service: string;
  critical_count: number;
  high_count: number;
  medium_count: number;
}

// ── Crashes ─────────────────────────────────────────────────────

export async function fetchCrashes(page = 1, size = 20, severity = '', trigger = ''): Promise<PaginatedResult<CrashSummary>> {
  const params = new URLSearchParams({ page: String(page), size: String(size) });
  if (severity) params.set('severity', severity);
  if (trigger) params.set('trigger', trigger);
  const res = await fetch(`${API_BASE}/crashes?${params}`);
  if (!res.ok) throw new Error('Failed to fetch crashes');
  return res.json();
}

// ── Attacks ─────────────────────────────────────────────────────

export async function fetchAttacks(page = 1, size = 20, severity = '', service = ''): Promise<PaginatedResult<AttackEntry>> {
  const params = new URLSearchParams({ page: String(page), size: String(size) });
  if (severity) params.set('severity', severity);
  if (service) params.set('service', service);
  const res = await fetch(`${API_BASE}/attacks?${params}`);
  if (!res.ok) throw new Error('Failed to fetch attacks');
  return res.json();
}

// ── Stats ───────────────────────────────────────────────────────

export async function fetchStats(): Promise<AggregatedStats> {
  const res = await fetch(`${API_BASE}/stats`);
  if (!res.ok) throw new Error('Failed to fetch stats');
  return res.json();
}
