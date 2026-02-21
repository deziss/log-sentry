import { useQuery } from '@tanstack/react-query';
import { fetchServices, fetchHealth, fetchRules } from '../api';
import { Activity, Shield, Server, AlertTriangle, Flame } from 'lucide-react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';

const COLORS = ['#3b82f6', '#f97316', '#ef4444', '#eab308', '#22c55e', '#8b5cf6'];

interface CrashSummary {
  id: string; started_at: string; trigger: string; severity: string; resolved: boolean;
}

export default function Dashboard() {
  const { data: services, isLoading: loadingSvc } = useQuery({ queryKey: ['services'], queryFn: fetchServices });
  const { data: health } = useQuery({ queryKey: ['health'], queryFn: fetchHealth });
  const { data: rules } = useQuery({ queryKey: ['rules'], queryFn: fetchRules });
  const { data: crashes } = useQuery<CrashSummary[]>({
    queryKey: ['crashes'],
    queryFn: () => fetch('/api/crashes').then(r => r.json()),
    refetchInterval: 5000,
  });

  const enabledServices = services?.filter(s => s.enabled) || [];
  const disabledServices = services?.filter(s => !s.enabled) || [];
  const recentCrashes = (crashes || []).slice(-3).reverse();

  const typeCount: Record<string, number> = {};
  services?.forEach(s => { typeCount[s.type] = (typeCount[s.type] || 0) + 1; });
  const pieData = Object.entries(typeCount).map(([name, value]) => ({ name, value }));
  const barData = Object.entries(typeCount).map(([type, count]) => ({ type, count }));

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-theme-heading">Dashboard</h1>
        <p className="text-theme-secondary text-sm mt-1">Real-time overview of your security posture</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={<Server className="w-5 h-5" />} label="Active Services" value={enabledServices.length} sub={`${disabledServices.length} disabled`} color="text-accent-light" loading={loadingSvc} />
        <StatCard icon={<Activity className="w-5 h-5" />} label="Available Parsers" value={health?.parsers || 0} sub="registered types" color="text-green-400" />
        <StatCard icon={<Shield className="w-5 h-5" />} label="Process Blacklist" value={rules?.ProcessBlacklist?.length || 0} sub="watched processes" color="text-orange-400" />
        <StatCard icon={<AlertTriangle className="w-5 h-5" />} label="System Status" value={health?.status === 'ok' ? 'Healthy' : 'Error'} sub="all systems" color={health?.status === 'ok' ? 'text-green-400' : 'text-red-400'} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-theme-secondary mb-4">Service Type Distribution</h3>
          {pieData.length > 0 ? (
            <ResponsiveContainer width="100%" height={240}>
              <PieChart>
                <Pie data={pieData} cx="50%" cy="50%" outerRadius={80} dataKey="value" label={({ name, value }) => `${name} (${value})`}>
                  {pieData.map((_, i) => <Cell key={i} fill={COLORS[i % COLORS.length]} />)}
                </Pie>
                <Tooltip contentStyle={{ background: 'var(--tooltip-bg)', border: '1px solid var(--tooltip-border)', borderRadius: '8px', color: 'var(--text-primary)' }} />
              </PieChart>
            </ResponsiveContainer>
          ) : <div className="h-60 flex items-center justify-center text-theme-muted">No services configured</div>}
        </div>

        <div className="glass-card">
          <h3 className="text-sm font-semibold text-theme-secondary mb-4">Parser Usage</h3>
          {barData.length > 0 ? (
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={barData}>
                <XAxis dataKey="type" tick={{ fill: 'var(--text-secondary)', fontSize: 12 }} />
                <YAxis tick={{ fill: 'var(--text-secondary)', fontSize: 12 }} />
                <Tooltip contentStyle={{ background: 'var(--tooltip-bg)', border: '1px solid var(--tooltip-border)', borderRadius: '8px', color: 'var(--text-primary)' }} />
                <Bar dataKey="count" fill="#3b82f6" radius={[6, 6, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : <div className="h-60 flex items-center justify-center text-theme-muted">No data</div>}
        </div>
      </div>

      {/* Recent Crashes */}
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-theme-secondary mb-4 flex items-center gap-2">
          <Flame className="w-4 h-4 text-orange-400" /> Recent Crash Events
        </h3>
        {recentCrashes.length > 0 ? (
          <div className="space-y-2">
            {recentCrashes.map(c => (
              <div key={c.id} className="flex items-center justify-between p-3 rounded-lg border" style={{ background: 'var(--bg-primary)', borderColor: 'var(--border-subtle)' }}>
                <div className="flex items-center gap-3">
                  <div className={`w-2 h-2 rounded-full ${c.resolved ? 'bg-green-400' : 'bg-red-400 animate-pulse'}`} />
                  <span className="text-sm font-medium text-theme-heading">{c.trigger}</span>
                  <span className={`badge badge-${c.severity}`}>{c.severity}</span>
                </div>
                <span className="text-xs text-theme-muted">{new Date(c.started_at).toLocaleString()}</span>
              </div>
            ))}
          </div>
        ) : <div className="text-center py-6 text-theme-muted">No crash events recorded</div>}
      </div>

      {/* Services List */}
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-theme-secondary mb-4">Monitored Services</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {services?.map(svc => (
            <div key={svc.name} className="flex items-center gap-3 p-3 rounded-lg border" style={{ background: 'var(--bg-primary)', borderColor: 'var(--border-subtle)' }}>
              <div className={`w-2 h-2 rounded-full ${svc.enabled ? 'bg-green-400' : 'bg-gray-600'}`} />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-theme-heading truncate">{svc.name}</p>
                <p className="text-xs text-theme-muted truncate">{svc.log_path}</p>
              </div>
              <span className="badge badge-ok">{svc.type}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function StatCard({ icon, label, value, sub, color, loading }: {
  icon: React.ReactNode; label: string; value: string | number; sub: string; color: string; loading?: boolean;
}) {
  return (
    <div className="glass-card">
      <div className="flex items-center gap-3 mb-3">
        <div className={`${color}`}>{icon}</div>
        <span className="text-xs text-theme-muted uppercase tracking-wider">{label}</span>
      </div>
      {loading ? <div className="skeleton h-8 w-20 mb-1" /> : <p className={`text-2xl font-bold ${color}`}>{value}</p>}
      <p className="text-xs text-theme-muted mt-1">{sub}</p>
    </div>
  );
}
