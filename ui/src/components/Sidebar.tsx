import { Shield, Server, ShieldAlert, Activity, Heart, Flame, Sun, Moon, ExternalLink, type LucideIcon } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { useTheme } from '../ThemeContext';
import { fetchConfig, type AppConfig } from '../api';
import type { Page } from '../App';

interface SidebarProps {
  current: Page;
  onNavigate: (page: Page) => void;
}

const navItems: { id: Page; label: string; icon: LucideIcon }[] = [
  { id: 'dashboard', label: 'Dashboard', icon: Activity },
  { id: 'services', label: 'Services', icon: Server },
  { id: 'rules', label: 'Rules', icon: Shield },
  { id: 'attacks', label: 'Attack Log', icon: ShieldAlert },
  { id: 'crashes', label: 'Crash Reports', icon: Flame },
  { id: 'health', label: 'Health', icon: Heart },
];

export default function Sidebar({ current, onNavigate }: SidebarProps) {
  const { theme, toggleTheme } = useTheme();
  const { data: config } = useQuery<AppConfig>({ queryKey: ['config'], queryFn: fetchConfig, staleTime: 60000, retry: false });

  return (
    <aside className="w-64 flex flex-col" style={{ background: 'var(--bg-secondary)', borderRight: '1px solid var(--border-subtle)' }}>
      {/* Logo */}
      <div className="p-6" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-accent to-blue-400 flex items-center justify-center">
            <Shield className="w-5 h-5 text-white" />
          </div>
          <div>
            <h1 className="text-lg font-bold text-theme-heading tracking-tight">Log Sentry</h1>
            <p className="text-xs text-theme-muted">Security Monitor</p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 p-4 space-y-1">
        {navItems.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => onNavigate(id)}
            className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-all duration-200 cursor-pointer
              ${current === id
                ? 'bg-accent/15 text-accent-light border border-accent/20'
                : 'text-theme-secondary hover:text-theme-heading hover:bg-accent/5'
              }`}
          >
            <Icon className="w-4 h-4" />
            {label}
          </button>
        ))}
      </nav>

      {/* External Links */}
      {config && (
        <div className="px-4 pb-3 space-y-1">
          <p className="text-[10px] text-theme-muted uppercase tracking-wider font-semibold px-2 mb-2">External</p>
          {config.loki_url && (
            <a href={config.loki_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs text-theme-secondary hover:text-accent-light hover:bg-accent/5 transition-colors">
              <ExternalLink className="w-3 h-3" /> Loki
            </a>
          )}
          {config.prometheus_url && (
            <a href={config.prometheus_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs text-theme-secondary hover:text-accent-light hover:bg-accent/5 transition-colors">
              <ExternalLink className="w-3 h-3" /> Prometheus
            </a>
          )}
          {config.grafana_url && (
            <a href={config.grafana_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs text-theme-secondary hover:text-accent-light hover:bg-accent/5 transition-colors">
              <ExternalLink className="w-3 h-3" /> Grafana
            </a>
          )}
        </div>
      )}

      {/* Footer */}
      <div className="p-4 flex items-center justify-between" style={{ borderTop: '1px solid var(--border-subtle)' }}>
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
          <span className="text-xs text-theme-muted">Agent Online</span>
        </div>
        <button onClick={toggleTheme} className="p-2 rounded-lg hover:bg-accent/10 text-theme-secondary hover:text-accent-light transition-colors cursor-pointer" title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}>
          {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
        </button>
      </div>
    </aside>
  );
}
