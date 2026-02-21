import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useReactTable, getCoreRowModel, flexRender, createColumnHelper } from '@tanstack/react-table';
import { fetchServices, fetchParsers, addService, updateService, deleteService, toggleService, type ServiceDef } from '../api';
import { Plus, Pencil, Trash2, X, Loader2 } from 'lucide-react';
import { useToast } from '../components/Toast';

const col = createColumnHelper<ServiceDef>();

export default function Services() {
  const qc = useQueryClient();
  const { addToast } = useToast();
  const { data: services = [] } = useQuery({ queryKey: ['services'], queryFn: fetchServices });
  const { data: parsers = [] } = useQuery({ queryKey: ['parsers'], queryFn: fetchParsers });
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<ServiceDef | null>(null);

  const addMut = useMutation({
    mutationFn: addService,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['services'] }); setShowModal(false); addToast('Service added successfully'); },
    onError: (e: Error) => addToast(e.message, 'error'),
  });
  const updateMut = useMutation({
    mutationFn: ({ name, svc }: { name: string; svc: ServiceDef }) => updateService(name, svc),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['services'] }); setEditing(null); addToast('Service updated'); },
    onError: (e: Error) => addToast(e.message, 'error'),
  });
  const deleteMut = useMutation({
    mutationFn: deleteService,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['services'] }); addToast('Service deleted'); },
    onError: (e: Error) => addToast(e.message, 'error'),
  });
  const toggleMut = useMutation({
    mutationFn: ({ name, enabled }: { name: string; enabled: boolean }) => toggleService(name, enabled),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['services'] }); addToast('Service toggled'); },
    onError: (e: Error) => addToast(e.message, 'error'),
  });

  const columns = [
    col.accessor('name', { header: 'Name', cell: info => <span className="font-medium text-theme-heading">{info.getValue()}</span> }),
    col.accessor('type', { header: 'Type', cell: info => <span className="badge badge-ok">{info.getValue()}</span> }),
    col.accessor('log_path', { header: 'Log Path', cell: info => <span className="text-theme-muted text-xs font-mono">{info.getValue()}</span> }),
    col.accessor('enabled', {
      header: 'Status',
      cell: info => {
        const svc = info.row.original;
        return (
          <div
            className={`toggle-switch ${info.getValue() ? 'active' : ''}`}
            onClick={(e) => { e.stopPropagation(); toggleMut.mutate({ name: svc.name, enabled: !svc.enabled }); }}
            title={info.getValue() ? 'Click to disable' : 'Click to enable'}
          />
        );
      },
    }),
    col.display({
      id: 'actions',
      header: 'Actions',
      cell: ({ row }) => (
        <div className="flex gap-2">
          <button onClick={() => setEditing(row.original)} className="p-1.5 rounded-lg hover:bg-accent/10 text-theme-muted hover:text-accent-light transition-colors cursor-pointer">
            <Pencil className="w-4 h-4" />
          </button>
          <button onClick={() => { if (confirm(`Delete "${row.original.name}"?`)) deleteMut.mutate(row.original.name); }} className="p-1.5 rounded-lg hover:bg-red-500/10 text-theme-muted hover:text-red-400 transition-colors cursor-pointer">
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      ),
    }),
  ];

  const table = useReactTable({ data: services, columns, getCoreRowModel: getCoreRowModel() });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-theme-heading">Services</h1>
          <p className="text-theme-secondary text-sm mt-1">Manage monitored log sources</p>
        </div>
        <button onClick={() => setShowModal(true)} className="flex items-center gap-2 px-4 py-2.5 bg-accent hover:bg-accent/80 text-white rounded-lg text-sm font-medium transition-colors cursor-pointer">
          <Plus className="w-4 h-4" /> Add Service
        </button>
      </div>

      <div className="glass-card overflow-x-auto">
        <table className="w-full">
          <thead>
            {table.getHeaderGroups().map(hg => (
              <tr key={hg.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                {hg.headers.map(h => (
                  <th key={h.id} className="text-left px-4 py-3 text-xs font-semibold text-theme-muted uppercase tracking-wider">
                    {flexRender(h.column.columnDef.header, h.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map(row => (
              <tr key={row.id} className="hover:bg-accent/5 transition-colors" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                {row.getVisibleCells().map(cell => (
                  <td key={cell.id} className="px-4 py-3">
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {services.length === 0 && <div className="py-12 text-center text-theme-muted">No services configured. Click "Add Service" to get started.</div>}
      </div>

      {showModal && <ServiceModal parsers={parsers} existingNames={services.map(s => s.name)} onClose={() => setShowModal(false)} onSave={(svc) => addMut.mutate(svc)} title="Add Service" loading={addMut.isPending} />}
      {editing && <ServiceModal parsers={parsers} existingNames={services.filter(s => s.name !== editing.name).map(s => s.name)} initial={editing} onClose={() => setEditing(null)} onSave={(svc) => updateMut.mutate({ name: editing.name, svc })} title="Edit Service" loading={updateMut.isPending} />}
    </div>
  );
}

function ServiceModal({ parsers, existingNames, initial, onClose, onSave, title, loading }: {
  parsers: string[]; existingNames: string[]; initial?: ServiceDef; onClose: () => void; onSave: (svc: ServiceDef) => void; title: string; loading: boolean;
}) {
  const [form, setForm] = useState<ServiceDef>(initial || { name: '', type: parsers[0] || 'nginx', log_path: '', enabled: true });
  const [touched, setTouched] = useState<Record<string, boolean>>({});

  const errors: Record<string, string> = {};
  if (!form.name.trim()) errors.name = 'Name is required';
  else if (existingNames.includes(form.name.trim())) errors.name = 'Name already exists';
  if (!form.log_path.trim()) errors.log_path = 'Log path is required';
  else if (!form.log_path.startsWith('/')) errors.log_path = 'Must be an absolute path (start with /)';
  const isValid = Object.keys(errors).length === 0;

  const touch = (field: string) => setTouched(prev => ({ ...prev, [field]: true }));

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="glass-card w-full max-w-md space-y-5">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-bold text-theme-heading">{title}</h2>
          <button onClick={onClose} className="p-1 hover:bg-accent/10 rounded-lg cursor-pointer"><X className="w-5 h-5 text-theme-muted" /></button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-xs text-theme-muted mb-1.5">Name</label>
            <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} onBlur={() => touch('name')} placeholder="web-1" className="w-full input-theme rounded-lg px-3 py-2.5 text-sm outline-none" />
            {touched.name && errors.name && <p className="text-xs text-red-400 mt-1">{errors.name}</p>}
          </div>
          <div>
            <label className="block text-xs text-theme-muted mb-1.5">Parser Type</label>
            <select value={form.type} onChange={e => setForm({ ...form, type: e.target.value })} className="w-full input-theme rounded-lg px-3 py-2.5 text-sm outline-none">
              {parsers.map(p => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs text-theme-muted mb-1.5">Log Path</label>
            <input value={form.log_path} onChange={e => setForm({ ...form, log_path: e.target.value })} onBlur={() => touch('log_path')} placeholder="/var/log/nginx/access.log" className="w-full input-theme rounded-lg px-3 py-2.5 text-sm outline-none" />
            {touched.log_path && errors.log_path && <p className="text-xs text-red-400 mt-1">{errors.log_path}</p>}
          </div>
          <label className="flex items-center gap-3 cursor-pointer">
            <input type="checkbox" checked={form.enabled} onChange={e => setForm({ ...form, enabled: e.target.checked })} className="w-4 h-4 rounded accent-accent" />
            <span className="text-sm text-theme-secondary">Enabled</span>
          </label>
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <button onClick={onClose} className="px-4 py-2 text-sm text-theme-muted hover:text-theme-heading transition-colors cursor-pointer">Cancel</button>
          <button
            onClick={() => { if (isValid) onSave(form); else setTouched({ name: true, log_path: true }); }}
            disabled={loading}
            className="flex items-center gap-2 px-4 py-2 bg-accent hover:bg-accent/80 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors cursor-pointer"
          >
            {loading && <Loader2 className="w-4 h-4 animate-spin" />}
            Save
          </button>
        </div>
      </div>
    </div>
  );
}
