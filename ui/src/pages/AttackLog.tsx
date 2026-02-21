import { ShieldAlert, Info } from 'lucide-react';

export default function AttackLog() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-theme-heading">Attack Log</h1>
        <p className="text-theme-secondary text-sm mt-1">Security events detected by the analyzer</p>
      </div>

      <div className="glass-card">
        <div className="flex items-center gap-3 p-4 rounded-lg bg-accent/10 border border-accent/20 text-accent-light text-sm mb-4">
          <Info className="w-4 h-4 shrink-0" />
          <span>Attack events are sourced from Prometheus metrics. Use the Prometheus link in the sidebar to query historical data.</span>
        </div>

        <table className="w-full">
          <thead>
            <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
              <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Severity</th>
              <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Type</th>
              <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Description</th>
              <th className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">Metric</th>
            </tr>
          </thead>
          <tbody>
            {attackTypes.map(atk => (
              <tr key={atk.type} className="hover:bg-accent/5 transition-colors" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                <td className="px-4 py-3"><span className={`badge badge-${atk.severity}`}>{atk.severity}</span></td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <ShieldAlert className="w-4 h-4 text-theme-muted" />
                    <span className="text-sm font-medium text-theme-heading">{atk.type}</span>
                  </div>
                </td>
                <td className="px-4 py-3 text-sm text-theme-secondary">{atk.description}</td>
                <td className="px-4 py-3"><code className="text-xs text-accent-light bg-accent/10 px-2 py-1 rounded">{atk.metric}</code></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

const attackTypes = [
  { type: 'SQL Injection', severity: 'critical', description: 'Detects UNION SELECT, OR 1=1, and encoded variants', metric: 'web_attack_detected_total{type="SQL Injection"}' },
  { type: 'XSS', severity: 'high', description: 'Detects <script>, javascript:, and encoded payloads', metric: 'web_attack_detected_total{type="XSS"}' },
  { type: 'Path Traversal', severity: 'high', description: 'Detects ../ directory traversal attempts', metric: 'web_attack_detected_total{type="Path Traversal"}' },
  { type: 'Scanner', severity: 'medium', description: 'Detects nikto, sqlmap, nmap user-agents', metric: 'web_attack_detected_total{type="Scanner"}' },
  { type: 'Data Exfiltration', severity: 'critical', description: 'Unusually large response body detected', metric: 'web_attack_detected_total{type="Data Exfiltration"}' },
  { type: '404 Flood', severity: 'high', description: 'IP exceeds rate limit for 404 responses', metric: 'web_anomaly_detected_total{type="404_flood"}' },
  { type: '500 Burst', severity: 'high', description: 'IP triggers excessive server errors', metric: 'web_anomaly_detected_total{type="500_burst"}' },
  { type: 'FIM Violation', severity: 'critical', description: 'Monitored file was modified', metric: 'sensitive_file_changed_total' },
  { type: 'Suspicious Process', severity: 'critical', description: 'Blacklisted process detected on host', metric: 'security_unexpected_process_active' },
];
