import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchRules, updateRules } from '../api';
import { X, Plus, Save, ShieldCheck, Loader2 } from 'lucide-react';
import { useToast } from '../components/Toast';

export default function RulesPage() {
  const qc = useQueryClient();
  const { addToast } = useToast();
  const { data: rules, isLoading } = useQuery({ queryKey: ['rules'], queryFn: fetchRules });
  const [blacklist, setBlacklist] = useState<string[]>([]);
  const [newItem, setNewItem] = useState('');
  const [initialized, setInitialized] = useState(false);

  if (rules && !initialized) {
    setBlacklist(rules.ProcessBlacklist || []);
    setInitialized(true);
  }

  const saveMut = useMutation({
    mutationFn: () => updateRules({ ProcessBlacklist: blacklist }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['rules'] }); addToast('Rules saved and hot-reloaded'); },
    onError: (e: Error) => addToast(e.message, 'error'),
  });

  const hasChanges = initialized && JSON.stringify(blacklist) !== JSON.stringify(rules?.ProcessBlacklist || []);

  const addItem = () => {
    const item = newItem.trim().toLowerCase();
    if (!item) { addToast('Process name cannot be empty', 'error'); return; }
    if (blacklist.includes(item)) { addToast(`"${item}" already in blacklist`, 'error'); return; }
    setBlacklist([...blacklist, item]);
    setNewItem('');
    addToast(`Added "${item}" to blacklist`, 'info');
  };

  const removeItem = (item: string) => {
    if (!confirm(`Remove "${item}" from blacklist?`)) return;
    setBlacklist(blacklist.filter(b => b !== item));
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-theme-heading">Rules</h1>
        <div className="glass-card"><div className="skeleton h-40 w-full" /></div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-theme-heading">Rules Configuration</h1>
          <p className="text-theme-secondary text-sm mt-1">Edit security rules — changes hot-reload automatically</p>
        </div>
        <button
          onClick={() => saveMut.mutate()}
          disabled={saveMut.isPending || !hasChanges}
          className="flex items-center gap-2 px-4 py-2.5 bg-green-600 hover:bg-green-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors cursor-pointer"
        >
          {saveMut.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
          {saveMut.isPending ? 'Saving...' : hasChanges ? 'Save Rules' : 'No Changes'}
        </button>
      </div>

      {saveMut.isSuccess && (
        <div className="flex items-center gap-2 p-3 rounded-lg bg-green-500/10 border border-green-500/20 text-green-400 text-sm">
          <ShieldCheck className="w-4 h-4" /> Rules saved and hot-reloaded successfully.
        </div>
      )}

      <div className="glass-card">
        <h3 className="text-sm font-semibold text-theme-secondary mb-4">Process Blacklist</h3>
        <p className="text-xs text-theme-muted mb-4">Processes matching these names will trigger security alerts.</p>

        <div className="flex flex-wrap gap-2 mb-4">
          {blacklist.map(item => (
            <span key={item} className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-red-500/15 border border-red-500/20 text-red-300 text-sm font-medium">
              {item}
              <button onClick={() => removeItem(item)} className="hover:text-red-100 transition-colors cursor-pointer">
                <X className="w-3.5 h-3.5" />
              </button>
            </span>
          ))}
          {blacklist.length === 0 && <span className="text-sm text-theme-muted">No processes in blacklist</span>}
        </div>

        <div className="flex gap-2">
          <input
            value={newItem}
            onChange={e => setNewItem(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && addItem()}
            placeholder="Add process name (e.g. cryptominer)"
            className="flex-1 input-theme rounded-lg px-3 py-2.5 text-sm placeholder-gray-500 outline-none"
          />
          <button onClick={addItem} className="flex items-center gap-2 px-4 py-2.5 bg-accent hover:bg-accent/80 text-white rounded-lg text-sm font-medium transition-colors cursor-pointer">
            <Plus className="w-4 h-4" /> Add
          </button>
        </div>
      </div>
    </div>
  );
}
