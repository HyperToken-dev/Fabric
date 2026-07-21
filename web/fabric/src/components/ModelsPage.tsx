import { useEffect, useState, type FormEvent } from 'react';
import { Plus, Sparkles } from 'lucide-react';
import { listChannels, type Channel } from '../api/channels';
import { enumLabel } from '../api/format';
import {
    createModel,
    listCatalogModels,
    listModels,
    MODEL_STATUSES,
    MODEL_TYPES,
    type CatalogModel,
    type Model,
} from '../api/models';
import {
    buttonClass,
    EmptyState,
    ErrorState,
    inputClass,
    LoadingCards,
    PageHeader,
    RefreshWarning,
} from './PageState';

export default function ModelsPage() {
    const [channels, setChannels] = useState<Channel[] | null>(null);
    const [channelId, setChannelId] = useState<number | null>(null);
    const [models, setModels] = useState<Model[] | null>(null);
    const [catalogModels, setCatalogModels] = useState<CatalogModel[] | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [version, setVersion] = useState(0);
    const [modelName, setModelName] = useState('');
    const [catalogModelName, setCatalogModelName] = useState('');
    const [status, setStatus] = useState(1);
    const [modelType, setModelType] = useState(1);
    const [saving, setSaving] = useState(false);
    const [addingCatalog, setAddingCatalog] = useState(false);

    const selectedChannel = channels?.find((channel) => channel.channelId === channelId) ?? null;
    const supportsCatalog = selectedChannel?.apiFormat === 1;
    const availableCatalogModels =
        catalogModels?.filter(
            (catalogModel) => !models?.some((model) => model.modelName === catalogModel.modelName),
        ) ?? [];

    useEffect(() => {
        const controller = new AbortController();
        setError(null);
        listChannels(controller.signal)
            .then((result) => {
                setChannels(result);
                setChannelId((current) =>
                    current && result.some((channel) => channel.channelId === current)
                        ? current
                        : (result[0]?.channelId ?? null),
                );
            })
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted)
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'Unable to load channels',
                    );
            });
        return () => controller.abort();
    }, [version]);
    useEffect(() => {
        if (!channelId) {
            setModels([]);
            return;
        }
        const controller = new AbortController();
        setError(null);
        setModels(null);
        listModels(channelId, controller.signal)
            .then(setModels)
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted)
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'Unable to load models',
                    );
            });
        return () => controller.abort();
    }, [channelId, version]);

    useEffect(() => {
        if (!supportsCatalog || !selectedChannel) {
            setCatalogModels([]);
            setCatalogModelName('');
            return;
        }
        const controller = new AbortController();
        setError(null);
        setCatalogModels(null);
        listCatalogModels(selectedChannel.apiFormat, controller.signal)
            .then((result) => {
                setCatalogModels(result);
                setCatalogModelName((current) =>
                    current && result.some((model) => model.modelName === current)
                        ? current
                        : (result[0]?.modelName ?? ''),
                );
            })
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted)
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'Unable to load catalog models',
                    );
            });
        return () => controller.abort();
    }, [selectedChannel, supportsCatalog, version]);

    useEffect(() => {
        if (!supportsCatalog) return;
        const available =
            catalogModels?.filter(
                (catalogModel) =>
                    !models?.some((model) => model.modelName === catalogModel.modelName),
            ) ?? [];
        setCatalogModelName((current) =>
            current && available.some((model) => model.modelName === current)
                ? current
                : (available[0]?.modelName ?? ''),
        );
    }, [catalogModels, models, supportsCatalog]);

    async function submit(event: FormEvent) {
        event.preventDefault();
        if (!channelId || !modelName.trim() || saving) return;
        setSaving(true);
        setError(null);
        try {
            const model = await createModel({
                modelName: modelName.trim(),
                channelId,
                status,
                modelType,
            });
            setModels((current) => [...(current ?? []), model]);
            setModelName('');
        } catch (requestError) {
            setError(
                requestError instanceof Error ? requestError.message : 'Unable to create model',
            );
        } finally {
            setSaving(false);
        }
    }

    async function addCatalogModel() {
        if (!channelId || !catalogModelName || addingCatalog) return;
        const catalogModel = catalogModels?.find((model) => model.modelName === catalogModelName);
        if (!catalogModel || models?.some((model) => model.modelName === catalogModel.modelName))
            return;
        setAddingCatalog(true);
        setError(null);
        try {
            const model = await createModel({
                modelName: catalogModel.modelName,
                channelId,
                status: 1,
                modelType: catalogModel.modelType,
            });
            setModels((current) => [...(current ?? []), model]);
            setCatalogModelName(
                availableCatalogModels.find((item) => item.modelName !== catalogModel.modelName)
                    ?.modelName ?? '',
            );
        } catch (requestError) {
            setError(
                requestError instanceof Error
                    ? requestError.message
                    : 'Unable to add catalog model',
            );
        } finally {
            setAddingCatalog(false);
        }
    }

    if (!channels && error)
        return (
            <div className="mx-auto max-w-7xl p-5 md:p-8">
                <ErrorState message={error} retry={() => setVersion((value) => value + 1)} />
            </div>
        );
    return (
        <div className="mx-auto max-w-7xl space-y-6 p-5 md:p-8">
            <PageHeader
                eyebrow="Model catalog"
                title="Models"
                description="Register model names under an upstream channel."
            />
            {error && channels && (
                <RefreshWarning message={error} retry={() => setVersion((value) => value + 1)} />
            )}
            {!channels && <LoadingCards />}
            {channels?.length === 0 && (
                <EmptyState
                    title="Create a channel first"
                    description="Models must belong to an existing upstream channel."
                />
            )}
            {!!channels?.length && (
                <>
                    <section className="overflow-hidden rounded-3xl border border-slate-100 bg-white shadow-sm">
                        <div className="border-b border-slate-100 bg-slate-50/70 p-5">
                            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
                                <label className="text-sm font-medium text-slate-700">
                                    Channel
                                    <select
                                        className={inputClass}
                                        value={channelId ?? ''}
                                        onChange={(event) =>
                                            setChannelId(Number(event.target.value))
                                        }
                                    >
                                        {channels.map((channel) => (
                                            <option
                                                key={channel.channelId}
                                                value={channel.channelId}
                                            >
                                                {channel.channelName}
                                            </option>
                                        ))}
                                    </select>
                                </label>
                                <div className="rounded-2xl border border-slate-200 bg-white px-4 py-3 text-sm text-slate-600">
                                    <span className="font-semibold text-slate-900">
                                        {selectedChannel?.channelName ?? 'No channel'}
                                    </span>
                                    <span className="mx-2 text-slate-300">/</span>
                                    {supportsCatalog ? 'OpenAI-compatible' : 'Custom API format'}
                                </div>
                            </div>
                        </div>
                        <div
                            className={`grid gap-4 p-5 ${supportsCatalog ? 'lg:grid-cols-[1.1fr_1fr]' : ''}`}
                        >
                            {supportsCatalog && (
                                <div className="relative overflow-hidden rounded-2xl border border-emerald-200 bg-gradient-to-br from-emerald-50 via-teal-50 to-cyan-50 p-5">
                                    <div className="absolute -right-10 -top-10 h-28 w-28 rounded-full bg-emerald-200/50 blur-2xl" />
                                    <div className="relative space-y-4">
                                        <div className="flex items-start justify-between gap-4">
                                            <div>
                                                <div className="mb-2 inline-flex items-center rounded-full bg-white/80 px-3 py-1 text-xs font-bold text-emerald-700 shadow-sm shadow-emerald-900/5">
                                                    <Sparkles size={13} className="mr-1.5" />
                                                    Official catalog
                                                </div>
                                                <h2 className="text-lg font-black text-emerald-950">
                                                    Pick a known OpenAI model
                                                </h2>
                                                <p className="mt-1 max-w-xl text-sm text-emerald-800">
                                                    Add official model names to this channel without
                                                    typing them manually.
                                                </p>
                                            </div>
                                            {catalogModels && (
                                                <span className="rounded-full bg-white/80 px-3 py-1 text-xs font-bold text-emerald-700 shadow-sm shadow-emerald-900/5">
                                                    {availableCatalogModels.length} available
                                                </span>
                                            )}
                                        </div>
                                        {catalogModels === null && (
                                            <div className="rounded-xl border border-white/70 bg-white/70 p-4 text-sm font-medium text-emerald-800">
                                                Loading catalog models...
                                            </div>
                                        )}
                                        {catalogModels !== null &&
                                            availableCatalogModels.length === 0 && (
                                                <div className="rounded-xl border border-white/70 bg-white/70 p-4 text-sm font-medium text-emerald-800">
                                                    All official catalog models are already
                                                    registered for this channel.
                                                </div>
                                            )}
                                        {availableCatalogModels.length > 0 && (
                                            <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
                                                <label className="text-sm font-bold text-emerald-950">
                                                    Official model
                                                    <select
                                                        className={`${inputClass} bg-white/90`}
                                                        value={catalogModelName}
                                                        onChange={(event) =>
                                                            setCatalogModelName(event.target.value)
                                                        }
                                                    >
                                                        {availableCatalogModels.map((model) => (
                                                            <option
                                                                key={model.modelName}
                                                                value={model.modelName}
                                                            >
                                                                {model.modelName}
                                                            </option>
                                                        ))}
                                                    </select>
                                                </label>
                                                <button
                                                    type="button"
                                                    disabled={addingCatalog}
                                                    onClick={() => void addCatalogModel()}
                                                    className="inline-flex min-h-11 items-center justify-center rounded-xl bg-emerald-600 px-4 py-2 text-sm font-black text-white shadow-lg shadow-emerald-900/15 transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
                                                >
                                                    <Plus size={16} className="mr-2" />
                                                    {addingCatalog ? 'Adding...' : 'Add official'}
                                                </button>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            )}
                            <form
                                onSubmit={submit}
                                className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm shadow-slate-900/5"
                            >
                                <div className="mb-4">
                                    <h2 className="text-lg font-black text-slate-950">
                                        Add a custom model
                                    </h2>
                                    <p className="mt-1 text-sm text-slate-500">
                                        Use any upstream-supported model name, including private or
                                        router-specific aliases.
                                    </p>
                                </div>
                                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_auto_auto] xl:items-end">
                                    <label className="text-sm font-medium text-slate-700 sm:col-span-2 xl:col-span-1">
                                        Model name
                                        <input
                                            className={inputClass}
                                            value={modelName}
                                            onChange={(event) => setModelName(event.target.value)}
                                            placeholder="Custom upstream model name"
                                            required
                                        />
                                    </label>
                                    <label className="text-sm font-medium text-slate-700">
                                        Status
                                        <select
                                            className={inputClass}
                                            value={status}
                                            onChange={(event) =>
                                                setStatus(Number(event.target.value))
                                            }
                                        >
                                            {Object.entries(MODEL_STATUSES).map(
                                                ([value, label]) => (
                                                    <option key={value} value={value}>
                                                        {label}
                                                    </option>
                                                ),
                                            )}
                                        </select>
                                    </label>
                                    <label className="text-sm font-medium text-slate-700">
                                        Type
                                        <select
                                            className={inputClass}
                                            value={modelType}
                                            onChange={(event) =>
                                                setModelType(Number(event.target.value))
                                            }
                                        >
                                            {Object.entries(MODEL_TYPES).map(([value, label]) => (
                                                <option key={value} value={value}>
                                                    {label}
                                                </option>
                                            ))}
                                        </select>
                                    </label>
                                </div>
                                <button disabled={saving} className={`${buttonClass} mt-4 w-full`}>
                                    <Plus size={16} className="mr-2" />
                                    {saving ? 'Adding...' : 'Add custom'}
                                </button>
                            </form>
                        </div>
                    </section>
                    {!models && <LoadingCards />}
                    {models?.length === 0 && (
                        <EmptyState
                            title="No models for this channel"
                            description="Register the upstream model names this channel can serve."
                        />
                    )}
                    {!!models?.length && (
                        <div className="overflow-hidden rounded-2xl border border-slate-100 bg-white shadow-sm">
                            <div className="overflow-x-auto">
                                <table className="w-full text-left text-sm">
                                    <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                                        <tr>
                                            <th className="px-5 py-3">Model</th>
                                            <th className="px-5 py-3">Status</th>
                                            <th className="px-5 py-3">Type</th>
                                            <th className="px-5 py-3">ID</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-slate-100">
                                        {models.map((model) => (
                                            <tr key={model.modelId}>
                                                <td className="px-5 py-4 font-semibold text-slate-900">
                                                    {model.modelName}
                                                </td>
                                                <td className="px-5 py-4">
                                                    {enumLabel(model.status, MODEL_STATUSES)}
                                                </td>
                                                <td className="px-5 py-4">
                                                    {enumLabel(model.modelType, MODEL_TYPES)}
                                                </td>
                                                <td className="px-5 py-4 text-slate-500">
                                                    {model.modelId}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    )}
                </>
            )}
        </div>
    );
}
