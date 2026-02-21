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
