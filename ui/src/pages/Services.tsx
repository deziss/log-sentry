import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useReactTable, getCoreRowModel, flexRender, createColumnHelper } from '@tanstack/react-table';
import { fetchServices, fetchParsers, addService, updateService, deleteService, type ServiceDef } from '../api';
import { Plus, Pencil, Trash2, X } from 'lucide-react';

const col = createColumnHelper<ServiceDef>();

export default function Services() {
  const qc = useQueryClient();
  const { data: services = [] } = useQuery({ queryKey: ['services'], queryFn: fetchServices });
  const { data: parsers = [] } = useQuery({ queryKey: ['parsers'], queryFn: fetchParsers });
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<ServiceDef | null>(null);

  const addMut = useMutation({ mutationFn: addService, onSuccess: () => { qc.invalidateQueries({ queryKey: ['services'] }); setShowModal(false); } });
  const updateMut = useMutation({ mutationFn: ({ name, svc }: { name: string; svc: ServiceDef }) => updateService(name, svc), onSuccess: () => { qc.invalidateQueries({ queryKey: ['services'] }); setEditing(null); } });
  const deleteMut = useMutation({ mutationFn: deleteService, onSuccess: () => qc.invalidateQueries({ queryKey: ['services'] }) });

  const columns = [
    col.accessor('name', { header: 'Name', cell: info => <span className="font-medium text-white">{info.getValue()}</span> }),
    col.accessor('type', { header: 'Type', cell: info => <span className="badge badge-ok">{info.getValue()}</span> }),
    col.accessor('log_path', { header: 'Log Path', cell: info => <span className="text-gray-400 text-xs font-mono">{info.getValue()}</span> }),
    col.accessor('enabled', {
      header: 'Status',
      cell: info => (
        <span className={`badge ${info.getValue() ? 'badge-low' : 'badge-medium'}`}>
          {info.getValue() ? 'Enabled' : 'Disabled'}
        </span>
      ),
    }),
    col.display({
      id: 'actions',
      header: 'Actions',
      cell: ({ row }) => (
        <div className="flex gap-2">
          <button onClick={() => setEditing(row.original)} className="p-1.5 rounded-lg hover:bg-white/10 text-gray-400 hover:text-accent-light transition-colors cursor-pointer">
            <Pencil className="w-4 h-4" />
          </button>
          <button onClick={() => { if (confirm(`Delete "${row.original.name}"?`)) deleteMut.mutate(row.original.name); }} className="p-1.5 rounded-lg hover:bg-red-500/10 text-gray-400 hover:text-red-400 transition-colors cursor-pointer">
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
          <h1 className="text-2xl font-bold text-white">Services</h1>
          <p className="text-gray-400 text-sm mt-1">Manage monitored log sources</p>
        </div>
        <button onClick={() => setShowModal(true)} className="flex items-center gap-2 px-4 py-2.5 bg-accent hover:bg-accent/80 text-white rounded-lg text-sm font-medium transition-colors cursor-pointer">
          <Plus className="w-4 h-4" /> Add Service
        </button>
      </div>

      {/* Table */}
      <div className="glass-card overflow-x-auto">
        <table className="w-full">
          <thead>
            {table.getHeaderGroups().map(hg => (
              <tr key={hg.id} className="border-b border-white/5">
                {hg.headers.map(h => (
                  <th key={h.id} className="text-left px-4 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">
                    {flexRender(h.column.columnDef.header, h.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map(row => (
              <tr key={row.id} className="border-b border-white/5 hover:bg-white/3 transition-colors">
                {row.getVisibleCells().map(cell => (
                  <td key={cell.id} className="px-4 py-3">
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {services.length === 0 && (
          <div className="py-12 text-center text-gray-500">No services configured. Click "Add Service" to get started.</div>
        )}
      </div>

      {/* Add Modal */}
      {showModal && (
        <ServiceModal parsers={parsers} onClose={() => setShowModal(false)} onSave={(svc) => addMut.mutate(svc)} title="Add Service" />
      )}

      {/* Edit Modal */}
      {editing && (
        <ServiceModal parsers={parsers} initial={editing} onClose={() => setEditing(null)} onSave={(svc) => updateMut.mutate({ name: editing.name, svc })} title="Edit Service" />
      )}
    </div>
  );
}

function ServiceModal({ parsers, initial, onClose, onSave, title }: {
  parsers: string[]; initial?: ServiceDef; onClose: () => void; onSave: (svc: ServiceDef) => void; title: string;
}) {
  const [form, setForm] = useState<ServiceDef>(initial || { name: '', type: 'nginx', log_path: '', enabled: true });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="glass-card w-full max-w-md space-y-5">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-bold text-white">{title}</h2>
          <button onClick={onClose} className="p-1 hover:bg-white/10 rounded-lg cursor-pointer"><X className="w-5 h-5 text-gray-400" /></button>
        </div>

        <div className="space-y-4">
          <Field label="Name" value={form.name} onChange={v => setForm({ ...form, name: v })} placeholder="web-1" />
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">Parser Type</label>
            <select value={form.type} onChange={e => setForm({ ...form, type: e.target.value })} className="w-full bg-navy-900 border border-white/10 rounded-lg px-3 py-2.5 text-sm text-white focus:border-accent outline-none">
              {parsers.map(p => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
          <Field label="Log Path" value={form.log_path} onChange={v => setForm({ ...form, log_path: v })} placeholder="/var/log/nginx/access.log" />
          <label className="flex items-center gap-3 cursor-pointer">
            <input type="checkbox" checked={form.enabled} onChange={e => setForm({ ...form, enabled: e.target.checked })} className="w-4 h-4 rounded accent-accent" />
            <span className="text-sm text-gray-300">Enabled</span>
          </label>
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <button onClick={onClose} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition-colors cursor-pointer">Cancel</button>
          <button onClick={() => onSave(form)} className="px-4 py-2 bg-accent hover:bg-accent/80 text-white rounded-lg text-sm font-medium transition-colors cursor-pointer">Save</button>
        </div>
      </div>
    </div>
  );
}

function Field({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (v: string) => void; placeholder: string }) {
  return (
    <div>
      <label className="block text-xs text-gray-400 mb-1.5">{label}</label>
      <input value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder} className="w-full bg-navy-900 border border-white/10 rounded-lg px-3 py-2.5 text-sm text-white placeholder-gray-600 focus:border-accent outline-none" />
    </div>
  );
}
