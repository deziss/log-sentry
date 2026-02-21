import { useState, Fragment } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, Clock, TrendingUp, ChevronRight, Cpu, HardDrive, Crosshair } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar, Cell } from 'recharts';

interface CrashSummary {
  id: string; started_at: string; ended_at: string; trigger: string;
  verdict: string; severity: string; resolved: boolean; snapshot_count: number;
}
interface ProcessSnapshot {
  pid: number; user: string; name: string; cmd: string;
  cpu_pct: number; mem_pct: number; mem_rss_mb: number; gpu_mem_mb?: number; oom_score: number;
  read_bytes: number; write_bytes: number; net_ports?: string;
  net_rx_bytes: number; net_tx_bytes: number; is_external: boolean;
  fd_count: number; thread_count: number;
}
interface GPUSnapshot { id: number; util_pct: number; mem_used_mb: number; mem_total_mb: number; temp_c: number; }
interface TrendPoint { ts: string; value: number; }
interface ForensicReport {
  snapshot_count: number; time_range: string; verdict: string; severity: string;
  cpu_trend: TrendPoint[]; mem_trend: TrendPoint[];
  top_cpu: ProcessSnapshot[]; top_mem: ProcessSnapshot[];
  oom_leaders: ProcessSnapshot[]; gpus?: GPUSnapshot[]; spike_detected: boolean;
}
interface CrashDetail {
  event: { id: string; started_at: string; ended_at: string; trigger: string; verdict: string; severity: string; resolved: boolean; snapshots: any[]; process_details?: Record<number, { exe_path: string; logs: string }>; };
  report: ForensicReport;
}

const SEV_COLORS: Record<string, string> = { critical: 'border-red-500/30 bg-red-500/10', high: 'border-orange-500/30 bg-orange-500/10', medium: 'border-yellow-500/30 bg-yellow-500/10', unknown: 'border-gray-500/30 bg-gray-500/10' };
const SEV_TEXT: Record<string, string> = { critical: 'text-red-400', high: 'text-orange-400', medium: 'text-yellow-400', unknown: 'text-gray-400' };
const SEV_DOT: Record<string, string> = { critical: 'bg-red-500', high: 'bg-orange-500', medium: 'bg-yellow-500', unknown: 'bg-gray-500' };

const tooltipStyle = { background: 'var(--tooltip-bg)', border: '1px solid var(--tooltip-border)', borderRadius: '8px', color: 'var(--text-primary)' };

export default function CrashReports() {
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const { data: crashes } = useQuery<CrashSummary[]>({
    queryKey: ['crashes'], queryFn: () => fetch('/api/crashes').then(r => r.json()), refetchInterval: 5000,
  });

  const { data: detail } = useQuery<CrashDetail>({
    queryKey: ['crash-detail', selectedId],
    queryFn: () => fetch(`/api/crashes?id=${selectedId}`).then(r => r.json()),
    enabled: !!selectedId,
  });

  if (selectedId && detail) return <CrashDetailView detail={detail} onBack={() => setSelectedId(null)} />;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-theme-heading">📋 Crash Reports</h1>
        <p className="text-theme-secondary text-sm mt-1">
          Only recorded when CPU/MEM/DISK/GPU ≥ 90% — zero storage when healthy
        </p>
      </div>

      {(!crashes || crashes.length === 0) ? (
        <div className="glass-card text-center py-12">
          <div className="text-5xl mb-4">✅</div>
          <h3 className="text-lg font-semibold text-green-400">No Crash Events</h3>
          <p className="text-theme-muted text-sm mt-1">System has stayed below threshold. No data stored.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {[...crashes].reverse().map(ev => (
            <div key={ev.id} onClick={() => setSelectedId(ev.id)}
              className={`glass-card cursor-pointer hover:border-blue-500/30 transition-all border ${SEV_COLORS[ev.severity] || SEV_COLORS.unknown}`}>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className={`w-3 h-3 rounded-full ${SEV_DOT[ev.severity] || SEV_DOT.unknown} ${!ev.resolved ? 'animate-pulse' : ''}`} />
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-semibold text-theme-heading">{ev.trigger}</span>
                      <span className={`badge badge-${ev.severity}`}>{ev.severity}</span>
                      {!ev.resolved && <span className="badge badge-high">ACTIVE</span>}
                    </div>
                    <p className="text-xs text-theme-muted mt-0.5 max-w-xl truncate">{ev.verdict}</p>
                  </div>
                </div>
                <div className="flex items-center gap-3 shrink-0">
                  <div className="text-right">
                    <div className="text-xs text-theme-muted flex items-center gap-1">
                      <Clock className="w-3 h-3" /> {new Date(ev.started_at).toLocaleString()}
                    </div>
                    <div className="text-xs text-theme-muted">{ev.snapshot_count} snapshots</div>
                  </div>
                  <ChevronRight className="w-4 h-4 text-theme-muted" />
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function CrashDetailView({ detail, onBack }: { detail: CrashDetail; onBack: () => void }) {
  const { event, report } = detail;
  return (
    <div className="space-y-6">
      <button onClick={onBack} className="text-sm text-blue-400 hover:text-blue-300 flex items-center gap-1 cursor-pointer">
        ← Back to Crash Reports
      </button>
      <div>
        <h1 className="text-2xl font-bold text-theme-heading">Crash Event {event.id}</h1>
        <p className="text-theme-secondary text-sm">
          {new Date(event.started_at).toLocaleString()} → {new Date(event.ended_at).toLocaleString()}
          {' • '}{event.snapshots.length} snapshots
        </p>
      </div>

      {/* Verdict */}
      <div className={`glass-card border-2 ${SEV_COLORS[event.severity] || SEV_COLORS.unknown}`}>
        <div className="flex items-start gap-4">
          <Crosshair className={`w-6 h-6 shrink-0 ${SEV_TEXT[event.severity]}`} />
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span className={`badge badge-${event.severity}`}>{event.severity}</span>
              <span className="text-sm text-theme-secondary">Trigger: {event.trigger}</span>
              {report.spike_detected && <span className="badge badge-high"><TrendingUp className="w-3 h-3 mr-1" />SPIKE</span>}
            </div>
            <p className={`text-sm ${SEV_TEXT[event.severity]}`}>{report.verdict}</p>
          </div>
        </div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-theme-secondary mb-4 flex items-center gap-2">
            <Cpu className="w-4 h-4 text-blue-400" /> CPU Timeline
          </h3>
          {report.cpu_trend?.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <AreaChart data={report.cpu_trend}>
                <defs><linearGradient id="cpuG" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4} /><stop offset="95%" stopColor="#3b82f6" stopOpacity={0} /></linearGradient></defs>
                <XAxis dataKey="ts" tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                <YAxis domain={[0, 100]} tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                <Tooltip contentStyle={tooltipStyle} />
                <Area type="monotone" dataKey="value" stroke="#3b82f6" fill="url(#cpuG)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : <div className="h-40 flex items-center justify-center text-theme-muted">No data</div>}
        </div>
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-theme-secondary mb-4 flex items-center gap-2">
            <HardDrive className="w-4 h-4 text-green-400" /> Memory Timeline
          </h3>
          {report.mem_trend?.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <AreaChart data={report.mem_trend}>
                <defs><linearGradient id="memG" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#22c55e" stopOpacity={0.4} /><stop offset="95%" stopColor="#22c55e" stopOpacity={0} /></linearGradient></defs>
                <XAxis dataKey="ts" tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                <YAxis domain={[0, 100]} tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                <Tooltip contentStyle={tooltipStyle} />
                <Area type="monotone" dataKey="value" stroke="#22c55e" fill="url(#memG)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : <div className="h-40 flex items-center justify-center text-theme-muted">No data</div>}
        </div>
      </div>

      {/* Process Tables */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-theme-secondary mb-4">Top Memory at Crash</h3>
          <ProcTable procs={report.top_mem || []} details={event.process_details} />
        </div>
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-theme-secondary mb-4 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-red-400" /> OOM Kill Targets
          </h3>
          {report.oom_leaders?.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <BarChart data={report.oom_leaders.slice(0, 6)} layout="vertical">
                <XAxis type="number" domain={[0, 1000]} tick={{ fill: 'var(--text-secondary)', fontSize: 10 }} />
                <YAxis dataKey="name" type="category" width={90} tick={{ fill: 'var(--text-primary)', fontSize: 11 }} />
                <Tooltip contentStyle={tooltipStyle} />
                <Bar dataKey="oom_score" radius={[0, 6, 6, 0]}>
                  {report.oom_leaders.slice(0, 6).map((_, i) => <Cell key={i} fill={i === 0 ? '#ef4444' : i < 3 ? '#f97316' : '#3b82f6'} />)}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          ) : <div className="h-40 text-center text-theme-muted flex items-center justify-center">No data</div>}
        </div>
      </div>
    </div>
  );
}

function ProcTable({ procs, details }: { procs: ProcessSnapshot[], details?: Record<number, { exe_path: string; logs: string }> }) {
  const [expandedPid, setExpandedPid] = useState<number | null>(null);

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead><tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
          <th className="text-left px-2 py-1.5 text-xs text-theme-muted uppercase">Process</th>
          <th className="text-left px-2 py-1.5 text-xs text-theme-muted uppercase">User</th>
          <th className="text-right px-2 py-1.5 text-xs text-theme-muted uppercase">RSS</th>
          <th className="text-right px-2 py-1.5 text-xs text-theme-muted uppercase">MEM%</th>
          <th className="text-right px-2 py-1.5 text-xs text-theme-muted uppercase">OOM</th>
        </tr></thead>
        <tbody>
          {procs.slice(0, 10).map((p, i) => {
            const hasDetail = details && details[p.pid];
            const isExpanded = expandedPid === p.pid;
            return (
              <Fragment key={`${p.pid}-${i}`}>
                <tr className={`${hasDetail ? 'cursor-pointer hover:bg-accent/5' : ''}`} style={{ borderBottom: '1px solid var(--border-subtle)' }} onClick={() => hasDetail && setExpandedPid(isExpanded ? null : p.pid)}>
                  <td className="px-2 py-1.5 flex items-center gap-2">
                    {hasDetail && <ChevronRight className={`w-3 h-3 text-theme-muted transition-transform ${isExpanded ? 'rotate-90' : ''}`} />}
                    <span className="text-sm font-semibold text-theme-heading">{p.name}</span>
                    <span className="text-xs text-theme-muted ml-1">:{p.pid}</span>
                    <span className="ml-2 px-1.5 py-0.5 rounded text-[10px] font-mono bg-blue-500/10 text-blue-400 border border-blue-500/20">FDs: {p.fd_count}</span>
                    <span className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-purple-500/10 text-purple-400 border border-purple-500/20">Threads: {p.thread_count}</span>
                  </td>
                  <td className="px-2 py-1.5 text-sm text-theme-secondary">{p.user}</td>
                  <td className="px-2 py-1.5 text-sm text-right font-mono text-theme-primary">{p.mem_rss_mb.toFixed(0)}M</td>
                  <td className="px-2 py-1.5 text-sm text-right"><span className={p.mem_pct > 20 ? 'text-red-400 font-bold' : 'text-theme-primary'}>{p.mem_pct.toFixed(1)}%</span></td>
                  <td className="px-2 py-1.5 text-sm text-right"><span className={p.oom_score > 800 ? 'text-red-400 font-bold' : 'text-theme-secondary'}>{p.oom_score}</span></td>
                </tr>
                {isExpanded && hasDetail && (
                  <tr style={{ borderBottom: '1px solid var(--border-subtle)', background: 'var(--bg-primary)' }}>
                    <td colSpan={5} className="p-4">
                      <div className="space-y-3">
                        {details[p.pid].exe_path && (
                          <div>
                            <span className="text-xs text-theme-muted uppercase font-semibold">Executable Path</span>
                            <div className="font-mono text-xs text-blue-300 mt-1 break-all p-2 rounded border" style={{ background: 'var(--bg-secondary)', borderColor: 'var(--border-subtle)' }}>{details[p.pid].exe_path}</div>
                          </div>
                        )}
                        <div className="flex flex-wrap gap-6">
                          <div>
                            <span className="text-xs text-theme-muted uppercase font-semibold">Disk I/O </span>
                            <div className="font-mono text-xs text-green-300 mt-1">Read: {(p.read_bytes / 1024 / 1024).toFixed(3)} MB | Write: {(p.write_bytes / 1024 / 1024).toFixed(3)} MB</div>
                          </div>
                          <div>
                            <span className="text-xs text-theme-muted uppercase font-semibold">Network I/O </span>
                            <div className="font-mono text-xs text-cyan-300 mt-1">Rx: {(p.net_rx_bytes / 1024 / 1024).toFixed(3)} MB | Tx: {(p.net_tx_bytes / 1024 / 1024).toFixed(3)} MB</div>
                          </div>
                          {p.net_ports && (
                            <div>
                              <span className="text-xs text-theme-muted uppercase font-semibold">Active Ports</span>
                              <div className="flex items-center gap-2 mt-1">
                                <span className="font-mono text-xs text-yellow-300 break-all">{p.net_ports}</span>
                                {p.is_external && <span className="px-1.5 py-0.5 rounded bg-red-500/20 border border-red-500/30 text-red-400 text-[10px] font-bold uppercase tracking-wider animate-pulse">External</span>}
                              </div>
                            </div>
                          )}
                        </div>
                        {details[p.pid].logs && (
                          <div>
                            <span className="text-xs text-theme-muted uppercase font-semibold">Recent Journal Logs</span>
                            <pre className="font-mono text-[10px] text-theme-secondary mt-1 overflow-x-auto p-3 rounded border max-h-60 overflow-y-auto whitespace-pre-wrap" style={{ background: 'var(--bg-secondary)', borderColor: 'var(--border-subtle)' }}>
                              {details[p.pid].logs}
                            </pre>
                          </div>
                        )}
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
      {procs.length === 0 && <div className="py-6 text-center text-theme-muted">No data</div>}
    </div>
  );
}
