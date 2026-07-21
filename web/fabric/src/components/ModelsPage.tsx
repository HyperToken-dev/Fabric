import { useEffect, useState, type FormEvent } from 'react';
import { Plus } from 'lucide-react';
import { listChannels, type Channel } from '../api/channels';
import { enumLabel } from '../api/format';
import { createModel, listModels, MODEL_STATUSES, MODEL_TYPES, type Model } from '../api/models';
import { buttonClass, EmptyState, ErrorState, inputClass, LoadingCards, PageHeader, RefreshWarning } from './PageState';

export default function ModelsPage() {
  const [channels, setChannels] = useState<Channel[] | null>(null);
  const [channelId, setChannelId] = useState<number | null>(null);
  const [models, setModels] = useState<Model[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [version, setVersion] = useState(0);
  const [modelName, setModelName] = useState('');
  const [status, setStatus] = useState(1);
  const [modelType, setModelType] = useState(1);
  const [saving, setSaving] = useState(false);

  useEffect(() => { const controller = new AbortController(); setError(null); listChannels(controller.signal).then((result) => { setChannels(result); setChannelId((current) => current && result.some((channel) => channel.channelId === current) ? current : result[0]?.channelId ?? null); }).catch((requestError: unknown) => { if (!controller.signal.aborted) setError(requestError instanceof Error ? requestError.message : 'Unable to load channels'); }); return () => controller.abort(); }, [version]);
  useEffect(() => { if (!channelId) { setModels([]); return; } const controller = new AbortController(); setError(null); setModels(null); listModels(channelId, controller.signal).then(setModels).catch((requestError: unknown) => { if (!controller.signal.aborted) setError(requestError instanceof Error ? requestError.message : 'Unable to load models'); }); return () => controller.abort(); }, [channelId, version]);

  async function submit(event: FormEvent) { event.preventDefault(); if (!channelId || !modelName.trim() || saving) return; setSaving(true); setError(null); try { const model = await createModel({ modelName: modelName.trim(), channelId, status, modelType }); setModels((current) => [...(current ?? []), model]); setModelName(''); } catch (requestError) { setError(requestError instanceof Error ? requestError.message : 'Unable to create model'); } finally { setSaving(false); } }

  if (!channels && error) return <div className="mx-auto max-w-7xl p-5 md:p-8"><ErrorState message={error} retry={() => setVersion((value) => value + 1)} /></div>;
  return <div className="mx-auto max-w-7xl space-y-6 p-5 md:p-8"><PageHeader eyebrow="Model catalog" title="Models" description="Register model names under an upstream channel." />
    {error && channels && <RefreshWarning message={error} retry={() => setVersion((value) => value + 1)} />}
    {!channels && <LoadingCards />}
    {channels?.length === 0 && <EmptyState title="Create a channel first" description="Models must belong to an existing upstream channel." />}
    {!!channels?.length && <><section className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm"><div className="grid gap-4 lg:grid-cols-[1fr_2fr]"><label className="text-sm font-medium text-slate-700">Channel<select className={inputClass} value={channelId ?? ''} onChange={(event) => setChannelId(Number(event.target.value))}>{channels.map((channel) => <option key={channel.channelId} value={channel.channelId}>{channel.channelName}</option>)}</select></label><form onSubmit={submit} className="grid gap-3 sm:grid-cols-[1fr_auto_auto_auto] sm:items-end"><label className="text-sm font-medium text-slate-700">Model name<input className={inputClass} value={modelName} onChange={(event) => setModelName(event.target.value)} required /></label><label className="text-sm font-medium text-slate-700">Status<select className={inputClass} value={status} onChange={(event) => setStatus(Number(event.target.value))}>{Object.entries(MODEL_STATUSES).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><label className="text-sm font-medium text-slate-700">Type<select className={inputClass} value={modelType} onChange={(event) => setModelType(Number(event.target.value))}>{Object.entries(MODEL_TYPES).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><button disabled={saving} className={buttonClass}><Plus size={16} className="mr-2" />{saving ? 'Adding...' : 'Add'}</button></form></div></section>
      {!models && <LoadingCards />}{models?.length === 0 && <EmptyState title="No models for this channel" description="Register the upstream model names this channel can serve." />}
      {!!models?.length && <div className="overflow-hidden rounded-2xl border border-slate-100 bg-white shadow-sm"><div className="overflow-x-auto"><table className="w-full text-left text-sm"><thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500"><tr><th className="px-5 py-3">Model</th><th className="px-5 py-3">Status</th><th className="px-5 py-3">Type</th><th className="px-5 py-3">ID</th></tr></thead><tbody className="divide-y divide-slate-100">{models.map((model) => <tr key={model.modelId}><td className="px-5 py-4 font-semibold text-slate-900">{model.modelName}</td><td className="px-5 py-4">{enumLabel(model.status, MODEL_STATUSES)}</td><td className="px-5 py-4">{enumLabel(model.modelType, MODEL_TYPES)}</td><td className="px-5 py-4 text-slate-500">{model.modelId}</td></tr>)}</tbody></table></div></div>}</>}
  </div>;
}
