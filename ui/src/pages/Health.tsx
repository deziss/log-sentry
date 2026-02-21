import { useQuery } from '@tanstack/react-query';
import { fetchHealth, fetchServices, fetchRules, fetchConfig } from '../api';
import { Heart, Server, Shield, FileWarning, Cpu, Radio, ExternalLink } from 'lucide-react';

export default function HealthPage() {
  const { data: health } = useQuery({ queryKey: ['health'], queryFn: fetchHealth });
  const { data: services = [] } = useQuery({ queryKey: ['services'], queryFn: fetchServices });
  const { data: rules } = useQuery({ queryKey: ['rules'], queryFn: fetchRules });
  const { data: config } = useQuery({ queryKey: ['config'], queryFn: fetchConfig, retry: false });

  const isHealthy = health?.status === 'ok';

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-theme-heading">System Health</h1>
        <p className="text-theme-secondary text-sm mt-1">Overview of agent and monitoring subsystems</p>
      </div>

      <div className={`glass-card flex items-center gap-4 ${isHealthy ? 'border-green-500/20' : 'border-red-500/20'}`}>
        <div className={`w-14 h-14 rounded-xl flex items-center justify-center ${isHealthy ? 'bg-green-500/15' : 'bg-red-500/15'}`}>
          <Heart className={`w-7 h-7 ${isHealthy ? 'text-green-400' : 'text-red-400'}`} />
        </div>
        <div>
          <p className={`text-xl font-bold ${isHealthy ? 'text-green-400' : 'text-red-400'}`}>
            {isHealthy ? 'All Systems Operational' : 'System Degraded'}
          </p>
          <p className="text-sm text-theme-secondary">{health?.services} services configured • {health?.parsers} parsers available</p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <SubsystemCard icon={<Server className="w-5 h-5" />} name="Log Collector" status="Active" detail={`${services.filter(s => s.enabled).length} services monitored`} ok />
        <SubsystemCard icon={<Shield className="w-5 h-5" />} name="Threat Analyzer" status="Active" detail="SQLi, XSS, Path Traversal, Scanner" ok />
        <SubsystemCard icon={<Radio className="w-5 h-5" />} name="Anomaly Detector" status="Active" detail="Token Bucket rate limiting" ok />
        <SubsystemCard icon={<FileWarning className="w-5 h-5" />} name="File Integrity (FIM)" status="Watching" detail="/etc/passwd" ok />
        <SubsystemCard icon={<Cpu className="w-5 h-5" />} name="Process Sentinel" status="Scanning" detail={`${rules?.ProcessBlacklist?.length || 0} processes in blacklist`} ok />
        <SubsystemCard icon={<Radio className="w-5 h-5" />} name="Syslog Server" status="Listening" detail="UDP/TCP :5140" ok />
      </div>

      {/* External Service Status */}
      {config && (
        <div className="glass-card">
          <h3 className="text-sm font-semibold text-theme-secondary mb-4 flex items-center gap-2">
            <ExternalLink className="w-4 h-4" /> External Service Links
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            {config.loki_url && (
              <a href={config.loki_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-3 p-3 rounded-lg border transition-colors hover:bg-accent/5" style={{ background: 'var(--bg-primary)', borderColor: 'var(--border-subtle)' }}>
                <div className="w-8 h-8 rounded-lg bg-orange-500/15 flex items-center justify-center"><span className="text-orange-400 font-bold text-sm">L</span></div>
                <div><p className="text-sm font-medium text-theme-heading">Loki</p><p className="text-xs text-theme-muted">{config.loki_url}</p></div>
              </a>
            )}
            {config.prometheus_url && (
              <a href={config.prometheus_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-3 p-3 rounded-lg border transition-colors hover:bg-accent/5" style={{ background: 'var(--bg-primary)', borderColor: 'var(--border-subtle)' }}>
                <div className="w-8 h-8 rounded-lg bg-red-500/15 flex items-center justify-center"><span className="text-red-400 font-bold text-sm">P</span></div>
                <div><p className="text-sm font-medium text-theme-heading">Prometheus</p><p className="text-xs text-theme-muted">{config.prometheus_url}</p></div>
              </a>
            )}
            {config.grafana_url && (
              <a href={config.grafana_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-3 p-3 rounded-lg border transition-colors hover:bg-accent/5" style={{ background: 'var(--bg-primary)', borderColor: 'var(--border-subtle)' }}>
                <div className="w-8 h-8 rounded-lg bg-green-500/15 flex items-center justify-center"><span className="text-green-400 font-bold text-sm">G</span></div>
                <div><p className="text-sm font-medium text-theme-heading">Grafana</p><p className="text-xs text-theme-muted">{config.grafana_url}</p></div>
              </a>
            )}
          </div>
        </div>
      )}

      <div className="glass-card">
        <h3 className="text-sm font-semibold text-theme-secondary mb-4">Service Health Detail</h3>
        <div className="space-y-2">
          {services.map(svc => (
            <div key={svc.name} className="flex items-center justify-between p-3 rounded-lg border" style={{ background: 'var(--bg-primary)', borderColor: 'var(--border-subtle)' }}>
              <div className="flex items-center gap-3">
                <div className={`w-2 h-2 rounded-full ${svc.enabled ? 'bg-green-400 animate-pulse' : 'bg-gray-600'}`} />
                <span className="text-sm font-medium text-theme-heading">{svc.name}</span>
                <span className="badge badge-ok">{svc.type}</span>
              </div>
              <span className="text-xs text-theme-muted font-mono">{svc.log_path}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function SubsystemCard({ icon, name, status, detail, ok }: {
  icon: React.ReactNode; name: string; status: string; detail: string; ok: boolean;
}) {
  return (
    <div className="glass-card">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <div className={ok ? 'text-green-400' : 'text-red-400'}>{icon}</div>
          <span className="text-sm font-semibold text-theme-heading">{name}</span>
        </div>
        <span className={`badge ${ok ? 'badge-low' : 'badge-critical'}`}>{status}</span>
      </div>
      <p className="text-xs text-theme-muted">{detail}</p>
    </div>
  );
}
