import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ShieldAlert, ChevronLeft, ChevronRight, Filter, Globe, Clock } from 'lucide-react';
import { fetchAttacks, type PaginatedResult, type AttackEntry } from '../api';

export default function AttackLog() {
  const [page, setPage] = useState(1);
  const [sevFilter, setSevFilter] = useState('');

  const { data: result } = useQuery<PaginatedResult<AttackEntry>>({
    queryKey: ['attacks', page, sevFilter],
    queryFn: () => fetchAttacks(page, 20, sevFilter),
    refetchInterval: 5000,
  });

  const attacks = result?.items || [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-theme-heading">Attack Log</h1>
          <p className="text-theme-secondary text-sm mt-1">
            {result ? `${result.total} events recorded` : 'Loading...'} — persisted in BoltDB
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-theme-muted" />
          <select value={sevFilter} onChange={e => { setSevFilter(e.target.value); setPage(1); }}
            className="text-sm rounded-lg px-3 py-1.5"
            style={{ background: 'var(--card-bg)', color: 'var(--text-primary)', border: '1px solid var(--border-subtle)' }}>
            <option value="">All Severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
          </select>
        </div>
      </div>

      <div className="glass-card">
        {attacks.length === 0 ? (
          <div className="text-center py-12">
            <div className="text-5xl mb-4">🛡️</div>
            <h3 className="text-lg font-semibold text-green-400">No Attacks Detected</h3>
            <p className="text-theme-muted text-sm mt-1">
              {sevFilter ? `No ${sevFilter} attacks found.` : 'Attacks will appear here as they are detected in real-time.'}
            </p>
          </div>
        ) : (
          <>
            <table className="w-full">
              <thead>
                <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Time</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Severity</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Type</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Service</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Source</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Endpoint</th>
                </tr>
              </thead>
              <tbody>
                {attacks.map((atk) => (
                  <tr key={atk.id} className="hover:bg-accent/5 transition-colors" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    <td className="px-4 py-3 text-xs text-theme-muted whitespace-nowrap">
                      <div className="flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {new Date(atk.timestamp).toLocaleString()}
                      </div>
                    </td>
                    <td className="px-4 py-3"><span className={`badge badge-${atk.severity}`}>{atk.severity}</span></td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <ShieldAlert className="w-4 h-4 text-theme-muted" />
                        <span className="text-sm font-medium text-theme-heading">{atk.type}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-sm text-theme-secondary">{atk.service || '—'}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1">
                        <Globe className="w-3 h-3 text-theme-muted" />
                        <span className="text-sm text-accent-light">{atk.source_ip}</span>
                        {atk.country && <span className="text-xs text-theme-muted">({atk.country})</span>}
                      </div>
                    </td>
                    <td className="px-4 py-3"><code className="text-xs text-accent-light bg-accent/10 px-2 py-1 rounded">{atk.endpoint || '—'}</code></td>
                  </tr>
                ))}
              </tbody>
            </table>

            {/* Pagination */}
            {result && result.total_pages > 1 && (
              <div className="flex items-center justify-center gap-4 pt-4 pb-2">
                <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page <= 1}
                  className="flex items-center gap-1 text-sm px-3 py-1.5 rounded-lg disabled:opacity-30"
                  style={{ background: 'var(--card-bg)', color: 'var(--text-primary)', border: '1px solid var(--border-subtle)' }}>
                  <ChevronLeft className="w-4 h-4" /> Prev
                </button>
                <span className="text-sm text-theme-muted">Page {result.page} of {result.total_pages}</span>
                <button onClick={() => setPage(p => Math.min(result.total_pages, p + 1))} disabled={page >= result.total_pages}
                  className="flex items-center gap-1 text-sm px-3 py-1.5 rounded-lg disabled:opacity-30"
                  style={{ background: 'var(--card-bg)', color: 'var(--text-primary)', border: '1px solid var(--border-subtle)' }}>
                  Next <ChevronRight className="w-4 h-4" />
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {/* Attack type reference */}
      <div className="glass-card">
        <h3 className="text-sm font-semibold text-theme-secondary mb-3">Detection Rules Reference</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
          {attackTypes.map(atk => (
            <div key={atk.type} className="flex items-start gap-2 p-2 rounded-lg hover:bg-accent/5">
              <span className={`badge badge-${atk.severity} mt-0.5`}>{atk.severity}</span>
              <div>
                <div className="text-sm font-medium text-theme-heading">{atk.type}</div>
                <div className="text-xs text-theme-muted">{atk.description}</div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

const attackTypes = [
  { type: 'SQL Injection', severity: 'critical', description: 'UNION SELECT, OR 1=1, encoded variants' },
  { type: 'XSS', severity: 'high', description: '<script>, javascript:, encoded payloads' },
  { type: 'Path Traversal', severity: 'high', description: '../ directory traversal attempts' },
  { type: 'Scanner', severity: 'medium', description: 'nikto, sqlmap, nmap user-agents' },
  { type: 'Data Exfiltration', severity: 'critical', description: 'Unusually large response body' },
  { type: '404 Flood', severity: 'high', description: 'IP rate limit for 404 responses' },
  { type: '500 Burst', severity: 'high', description: 'IP triggers excessive server errors' },
  { type: 'FIM Violation', severity: 'critical', description: 'Monitored file modification' },
  { type: 'Suspicious Process', severity: 'critical', description: 'Blacklisted process active' },
];
