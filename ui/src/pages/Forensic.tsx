import { useQuery } from '@tanstack/react-query';
import { Crosshair, TrendingUp, Cpu, HardDrive, AlertTriangle } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar, Cell } from 'recharts';

interface TrendPoint { ts: string; value: number; }
interface ProcessSnapshot {
  pid: number; user: string; name: string; cmd: string;
  cpu_pct: number; mem_pct: number; mem_rss_mb: number;
  gpu_mem_mb?: number; oom_score: number;
}
interface GPUSnapshot { id: number; util_pct: number; mem_used_mb: number; mem_total_mb: number; temp_c: number; }
interface ForensicReport {
  snapshot_count: number; time_range: string; verdict: string; severity: string;
  cpu_trend: TrendPoint[]; mem_trend: TrendPoint[];
  top_cpu: ProcessSnapshot[]; top_mem: ProcessSnapshot[];
  oom_leaders: ProcessSnapshot[]; gpus?: GPUSnapshot[];
  spike_detected: boolean;
}

async function fetchForensic(): Promise<ForensicReport> {
  const res = await fetch('/api/forensic');
  if (!res.ok) throw new Error('Failed to fetch forensic report');
  return res.json();
}

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'border-red-500/30 bg-red-500/10',
  high: 'border-orange-500/30 bg-orange-500/10',
  medium: 'border-yellow-500/30 bg-yellow-500/10',
  unknown: 'border-gray-500/30 bg-gray-500/10',
};
const SEVERITY_TEXT: Record<string, string> = {
  critical: 'text-red-400', high: 'text-orange-400', medium: 'text-yellow-400', unknown: 'text-gray-400',
};

export default function ForensicPage() {
  const { data: report, isLoading } = useQuery({ queryKey: ['forensic'], queryFn: fetchForensic, refetchInterval: 5000 });

  if (isLoading || !report) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-white">Forensic Analysis</h1>
        <div className="glass-card"><div className="skeleton h-40 w-full" /></div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">🔍 Crash Forensic Analysis</h1>
        <p className="text-gray-400 text-sm mt-1">
          {report.snapshot_count} snapshots • {report.time_range}
        </p>
      </div>

      {/* Root Cause Verdict */}
      <div className={`glass-card border-2 ${SEVERITY_COLORS[report.severity] || SEVERITY_COLORS.unknown}`}>
        <div className="flex items-start gap-4">
          <div className={`w-12 h-12 rounded-xl flex items-center justify-center shrink-0 ${report.severity === 'critical' ? 'bg-red-500/20' : 'bg-orange-500/20'}`}>
            <Crosshair className={`w-6 h-6 ${SEVERITY_TEXT[report.severity] || SEVERITY_TEXT.unknown}`} />
          </div>
          <div>
            <div className="flex items-center gap-2 mb-2">
              <span className={`badge badge-${report.severity}`}>{report.severity}</span>
              {report.spike_detected && (
                <span className="badge badge-high">
                  <TrendingUp className="w-3 h-3 mr-1" /> SPIKE DETECTED
                </span>
              )}
            </div>
            <p className={`text-sm font-medium ${SEVERITY_TEXT[report.severity] || SEVERITY_TEXT.unknown}`}>
              {report.verdict}
            </p>
          </div>
        </div>
      </div>

      {/* Trend Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 flex items-center gap-2">
            <Cpu className="w-4 h-4 text-blue-400" /> CPU Usage Over Time
          </h3>
          {report.cpu_trend?.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={report.cpu_trend}>
                <defs>
                  <linearGradient id="cpuGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4} />
                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <XAxis dataKey="ts" tick={{ fill: '#94a3b8', fontSize: 10 }} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={{ fill: '#94a3b8', fontSize: 10 }} />
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid rgba(255,255,255,0.1)', borderRadius: '8px', color: '#e2e8f0' }} />
                <Area type="monotone" dataKey="value" stroke="#3b82f6" fill="url(#cpuGrad)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : <div className="h-50 flex items-center justify-center text-gray-500">No data yet</div>}
        </div>

        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 flex items-center gap-2">
            <HardDrive className="w-4 h-4 text-green-400" /> Memory Usage Over Time
          </h3>
          {report.mem_trend?.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={report.mem_trend}>
                <defs>
                  <linearGradient id="memGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#22c55e" stopOpacity={0.4} />
                    <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <XAxis dataKey="ts" tick={{ fill: '#94a3b8', fontSize: 10 }} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={{ fill: '#94a3b8', fontSize: 10 }} />
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid rgba(255,255,255,0.1)', borderRadius: '8px', color: '#e2e8f0' }} />
                <Area type="monotone" dataKey="value" stroke="#22c55e" fill="url(#memGrad)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : <div className="h-50 flex items-center justify-center text-gray-500">No data yet</div>}
        </div>
      </div>

      {/* GPU Status */}
      {report.gpus && report.gpus.length > 0 && (
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4">🎮 GPU Status (Last Snapshot)</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {report.gpus.map(gpu => (
              <div key={gpu.id} className="p-3 rounded-lg bg-navy-900/50 border border-white/5">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-white">GPU {gpu.id}</span>
                  <span className="text-xs text-gray-400">{gpu.temp_c}°C</span>
                </div>
                <div className="space-y-1">
                  <ProgressBar label="Utilization" value={gpu.util_pct} max={100} />
                  <ProgressBar label="Memory" value={gpu.mem_used_mb} max={gpu.mem_total_mb} suffix="MB" />
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Top Processes */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Top by Memory */}
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4">Top Memory Consumers</h3>
          <ProcessTable procs={report.top_mem || []} sortBy="mem" />
        </div>

        {/* OOM Leaders */}
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-gray-300 mb-4 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-red-400" /> OOM Kill Targets
          </h3>
          {report.oom_leaders?.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <BarChart data={report.oom_leaders.slice(0, 8)} layout="vertical">
                <XAxis type="number" domain={[0, 1000]} tick={{ fill: '#94a3b8', fontSize: 10 }} />
                <YAxis dataKey="name" type="category" width={100} tick={{ fill: '#e2e8f0', fontSize: 11 }} />
                <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid rgba(255,255,255,0.1)', borderRadius: '8px', color: '#e2e8f0' }} />
                <Bar dataKey="oom_score" radius={[0, 6, 6, 0]}>
                  {report.oom_leaders.slice(0, 8).map((_, i) => (
                    <Cell key={i} fill={i === 0 ? '#ef4444' : i < 3 ? '#f97316' : '#3b82f6'} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          ) : <div className="h-50 flex items-center justify-center text-gray-500">No data</div>}
        </div>
      </div>
    </div>
  );
}

function ProcessTable({ procs, sortBy }: { procs: ProcessSnapshot[]; sortBy: 'cpu' | 'mem' }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead>
          <tr className="border-b border-white/5">
            <th className="text-left px-3 py-2 text-xs text-gray-400 uppercase">Process</th>
            <th className="text-left px-3 py-2 text-xs text-gray-400 uppercase">User</th>
            <th className="text-right px-3 py-2 text-xs text-gray-400 uppercase">RSS (MB)</th>
            <th className="text-right px-3 py-2 text-xs text-gray-400 uppercase">MEM%</th>
            {sortBy === 'mem' && <th className="text-right px-3 py-2 text-xs text-gray-400 uppercase">OOM</th>}
          </tr>
        </thead>
        <tbody>
          {procs.slice(0, 10).map((p, i) => (
            <tr key={`${p.pid}-${i}`} className="border-b border-white/5 hover:bg-white/3">
              <td className="px-3 py-2">
                <div>
                  <span className="text-sm font-medium text-white">{p.name}</span>
                  <span className="text-xs text-gray-500 ml-2">PID:{p.pid}</span>
                </div>
              </td>
              <td className="px-3 py-2 text-sm text-gray-400">{p.user}</td>
              <td className="px-3 py-2 text-sm text-right font-mono text-gray-300">{p.mem_rss_mb.toFixed(0)}</td>
              <td className="px-3 py-2 text-sm text-right">
                <span className={p.mem_pct > 20 ? 'text-red-400 font-bold' : p.mem_pct > 10 ? 'text-orange-400' : 'text-gray-300'}>
                  {p.mem_pct.toFixed(1)}%
                </span>
              </td>
              {sortBy === 'mem' && (
                <td className="px-3 py-2 text-sm text-right">
                  <span className={p.oom_score > 800 ? 'text-red-400 font-bold' : p.oom_score > 500 ? 'text-orange-400' : 'text-gray-400'}>
                    {p.oom_score}
                  </span>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
      {procs.length === 0 && <div className="py-8 text-center text-gray-500">No process data</div>}
    </div>
  );
}

function ProgressBar({ label, value, max, suffix }: { label: string; value: number; max: number; suffix?: string }) {
  const pct = Math.min(100, (value / max) * 100);
  const color = pct > 90 ? 'bg-red-500' : pct > 70 ? 'bg-orange-500' : 'bg-blue-500';
  return (
    <div>
      <div className="flex justify-between text-xs text-gray-400 mb-0.5">
        <span>{label}</span>
        <span>{suffix ? `${value}/${max} ${suffix}` : `${pct.toFixed(0)}%`}</span>
      </div>
      <div className="h-1.5 bg-navy-900 rounded-full overflow-hidden">
        <div className={`h-full ${color} rounded-full transition-all duration-500`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
