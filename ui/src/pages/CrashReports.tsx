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
        <h1 className="text-2xl font-bold text-white">📋 Crash Reports</h1>
        <p className="text-gray-400 text-sm mt-1">
          Only recorded when CPU/MEM/DISK/GPU ≥ 90% — zero storage when healthy
        </p>
      </div>

      {(!crashes || crashes.length === 0) ? (
        <div className="glass-card text-center py-12">
          <div className="text-5xl mb-4">✅</div>
          <h3 className="text-lg font-semibold text-green-400">No Crash Events</h3>
          <p className="text-gray-500 text-sm mt-1">System has stayed below threshold. No data stored.</p>
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
                      <span className="text-sm font-semibold text-white">{ev.trigger}</span>
                      <span className={`badge badge-${ev.severity}`}>{ev.severity}</span>
                      {!ev.resolved && <span className="badge badge-high">ACTIVE</span>}
                    </div>
                    <p className="text-xs text-gray-400 mt-0.5 max-w-xl truncate">{ev.verdict}</p>
                  </div>
                </div>
                <div className="flex items-center gap-3 shrink-0">
                  <div className="text-right">
                    <div className="text-xs text-gray-400 flex items-center gap-1">
                      <Clock className="w-3 h-3" /> {new Date(ev.started_at).toLocaleString()}
                    </div>
                    <div className="text-xs text-gray-500">{ev.snapshot_count} snapshots</div>
                  </div>
                  <ChevronRight className="w-4 h-4 text-gray-500" />
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
      <button onClick={onBack} className="text-sm text-blue-400 hover:text-blue-300 flex items-center gap-1">
        ← Back to Crash Reports
      </button>
      <div>
        <h1 className="text-2xl font-bold text-white">Crash Event {event.id}</h1>
        <p className="text-gray-400 text-sm">
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
              <span className="text-sm text-gray-400">Trigger: {event.trigger}</span>
              {report.spike_detected && <span className="badge badge-high"><TrendingUp className="w-3 h-3 mr-1" />SPIKE</span>}
            </div>
            <p className={`text-sm ${SEV_TEXT[event.severity]}`}>{report.verdict}</p>
          </div>
        </div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 flex items-center gap-2">
            <Cpu className="w-4 h-4 text-blue-400" /> CPU Timeline
          </h3>
          {report.cpu_trend?.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <AreaChart data={report.cpu_trend}>
                <defs><linearGradient id="cpuG" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4} /><stop offset="95%" stopColor="#3b82f6" stopOpacity={0} /></linearGradient></defs>
                <XAxis dataKey="ts" tick={{ fill: '#94a3b8', fontSize: 10 }} />
                <YAxis domain={[0, 100]} tick={{ fill: '#94a3b8', fontSize: 10 }} />
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid rgba(255,255,255,0.1)', borderRadius: '8px', color: '#e2e8f0' }} />
                <Area type="monotone" dataKey="value" stroke="#3b82f6" fill="url(#cpuG)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : <div className="h-40 flex items-center justify-center text-gray-500">No data</div>}
        </div>
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 flex items-center gap-2">
            <HardDrive className="w-4 h-4 text-green-400" /> Memory Timeline
          </h3>
          {report.mem_trend?.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <AreaChart data={report.mem_trend}>
                <defs><linearGradient id="memG" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#22c55e" stopOpacity={0.4} /><stop offset="95%" stopColor="#22c55e" stopOpacity={0} /></linearGradient></defs>
                <XAxis dataKey="ts" tick={{ fill: '#94a3b8', fontSize: 10 }} />
                <YAxis domain={[0, 100]} tick={{ fill: '#94a3b8', fontSize: 10 }} />
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid rgba(255,255,255,0.1)', borderRadius: '8px', color: '#e2e8f0' }} />
                <Area type="monotone" dataKey="value" stroke="#22c55e" fill="url(#memG)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : <div className="h-40 flex items-center justify-center text-gray-500">No data</div>}
        </div>
      </div>

      {/* Process Tables */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4">Top Memory at Crash</h3>
          <ProcTable procs={report.top_mem || []} details={event.process_details} />
        </div>
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-red-400" /> OOM Kill Targets
          </h3>
          {report.oom_leaders?.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <BarChart data={report.oom_leaders.slice(0, 6)} layout="vertical">
                <XAxis type="number" domain={[0, 1000]} tick={{ fill: '#94a3b8', fontSize: 10 }} />
                <YAxis dataKey="name" type="category" width={90} tick={{ fill: '#e2e8f0', fontSize: 11 }} />
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid rgba(255,255,255,0.1)', borderRadius: '8px', color: '#e2e8f0' }} />
                <Bar dataKey="oom_score" radius={[0, 6, 6, 0]}>
                  {report.oom_leaders.slice(0, 6).map((_, i) => <Cell key={i} fill={i === 0 ? '#ef4444' : i < 3 ? '#f97316' : '#3b82f6'} />)}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          ) : <div className="h-40 text-center text-gray-500 flex items-center justify-center">No data</div>}
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
        <thead><tr className="border-b border-white/5">
          <th className="text-left px-2 py-1.5 text-xs text-gray-400 uppercase">Process</th>
          <th className="text-left px-2 py-1.5 text-xs text-gray-400 uppercase">User</th>
          <th className="text-right px-2 py-1.5 text-xs text-gray-400 uppercase">RSS</th>
          <th className="text-right px-2 py-1.5 text-xs text-gray-400 uppercase">MEM%</th>
          <th className="text-right px-2 py-1.5 text-xs text-gray-400 uppercase">OOM</th>
        </tr></thead>
        <tbody>
          {procs.slice(0, 10).map((p, i) => {
            const hasDetail = details && details[p.pid];
            const isExpanded = expandedPid === p.pid;
            return (
              <Fragment key={`${p.pid}-${i}`}>
                <tr className={`border-b border-white/5 ${hasDetail ? 'cursor-pointer hover:bg-white/5' : ''}`} onClick={() => hasDetail && setExpandedPid(isExpanded ? null : p.pid)}>
                  <td className="px-2 py-1.5 flex items-center gap-2">
                    {hasDetail && <ChevronRight className={`w-3 h-3 text-gray-500 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />}
                    <span className="text-sm text-white">{p.name}</span><span className="text-xs text-gray-500 ml-1">:{p.pid}</span>
                  </td>
                  <td className="px-2 py-1.5 text-sm text-gray-400">{p.user}</td>
                  <td className="px-2 py-1.5 text-sm text-right font-mono text-gray-300">{p.mem_rss_mb.toFixed(0)}M</td>
                  <td className="px-2 py-1.5 text-sm text-right"><span className={p.mem_pct > 20 ? 'text-red-400 font-bold' : 'text-gray-300'}>{p.mem_pct.toFixed(1)}%</span></td>
                  <td className="px-2 py-1.5 text-sm text-right"><span className={p.oom_score > 800 ? 'text-red-400 font-bold' : 'text-gray-400'}>{p.oom_score}</span></td>
                </tr>
                {isExpanded && hasDetail && (
                  <tr className="border-b border-white/5 bg-black/20">
                    <td colSpan={5} className="p-4">
                      <div className="space-y-3">
                        {details[p.pid].exe_path && (
                          <div>
                            <span className="text-xs text-gray-500 uppercase font-semibold">Executable Path</span>
                            <div className="font-mono text-xs text-blue-300 mt-1 break-all bg-black/40 p-2 rounded border border-white/5">{details[p.pid].exe_path}</div>
                          </div>
                        )}
                        {details[p.pid].logs && (
                          <div>
                            <span className="text-xs text-gray-500 uppercase font-semibold">Recent Journal Logs</span>
                            <pre className="font-mono text-[10px] text-gray-300 mt-1 overflow-x-auto bg-black/40 p-3 rounded border border-white/5 max-h-60 overflow-y-auto whitespace-pre-wrap">
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
      {procs.length === 0 && <div className="py-6 text-center text-gray-500">No data</div>}
    </div>
  );
}
