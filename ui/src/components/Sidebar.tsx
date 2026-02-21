import { Shield, Server, ShieldAlert, Activity, Heart, Crosshair, type LucideIcon } from 'lucide-react';

interface SidebarProps {
  current: string;
  onNavigate: (page: any) => void;
}

const navItems: { id: string; label: string; icon: LucideIcon }[] = [
  { id: 'dashboard', label: 'Dashboard', icon: Activity },
  { id: 'services', label: 'Services', icon: Server },
  { id: 'rules', label: 'Rules', icon: Shield },
  { id: 'attacks', label: 'Attack Log', icon: ShieldAlert },
  { id: 'forensic', label: 'Forensic', icon: Crosshair },
  { id: 'health', label: 'Health', icon: Heart },
];

export default function Sidebar({ current, onNavigate }: SidebarProps) {
  return (
    <aside className="w-64 bg-navy-800/80 backdrop-blur-xl border-r border-white/5 flex flex-col">
      {/* Logo */}
      <div className="p-6 border-b border-white/5">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-accent to-blue-400 flex items-center justify-center">
            <Shield className="w-5 h-5 text-white" />
          </div>
          <div>
            <h1 className="text-lg font-bold text-white tracking-tight">Log Sentry</h1>
            <p className="text-xs text-gray-400">Security Monitor</p>
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
                : 'text-gray-400 hover:text-gray-200 hover:bg-white/5'
              }`}
          >
            <Icon className="w-4 h-4" />
            {label}
          </button>
        ))}
      </nav>

      {/* Footer */}
      <div className="p-4 border-t border-white/5">
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
          <span className="text-xs text-gray-500">Agent Online</span>
        </div>
      </div>
    </aside>
  );
}
