import { useEffect, useState, type FormEvent } from 'react';
import { Check, Pencil, Plus, X } from 'lucide-react';
import {
    API_FORMAT_DEFAULT_BASE_URLS,
    API_FORMATS,
    createChannel,
    listChannels,
    updateChannelApiFormat,
    updateChannelBaseUrl,
    updateChannelName,
    updateChannelProviderKey,
    updateChannelStatus,
    type Channel,
} from '../api/channels';
import { useI18n } from '../i18n';
import {
    buttonClass,
    EmptyState,
    ErrorState,
    inputClass,
    LoadingCards,
    PageHeader,
    RefreshWarning,
} from './PageState';

type Draft = {
    channelName: string;
    baseUrl: string;
    status: number;
    apiFormat: number;
    providerKey: string;
};

const defaultAPIFormat = 1;
const channelStatuses = [1, 2, 3] as const;

function defaultBaseUrl(apiFormat: number): string {
    return API_FORMAT_DEFAULT_BASE_URLS[apiFormat] ?? '';
}

function newDraft(): Draft {
    return {
        channelName: '',
        baseUrl: defaultBaseUrl(defaultAPIFormat),
        status: 1,
        apiFormat: defaultAPIFormat,
        providerKey: '',
    };
}

export default function ChannelsPage() {
    const { t } = useI18n();
    const [channels, setChannels] = useState<Channel[] | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [version, setVersion] = useState(0);
    const [creating, setCreating] = useState(false);
    const [editing, setEditing] = useState<number | null>(null);
    const [saving, setSaving] = useState(false);
    const [draft, setDraft] = useState<Draft>(newDraft);

    useEffect(() => {
        const controller = new AbortController();
        setError(null);
        listChannels(controller.signal)
            .then(setChannels)
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted)
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'i18n:channels.loadError',
                    );
            });
        return () => controller.abort();
    }, [version]);

    function channelStatusLabel(status: number): string {
        if (status === 1) return t('status.active');
        if (status === 2) return t('status.banned');
        if (status === 3) return t('status.pending');
        return t('common.unknown', { value: status });
    }

    function apiFormatLabel(apiFormat: number): string {
        return (
            API_FORMATS[apiFormat as keyof typeof API_FORMATS] ??
            t('common.unknown', { value: apiFormat })
        );
    }

    function startEdit(channel: Channel) {
        setEditing(channel.channelId);
        setDraft({
            channelName: channel.channelName,
            baseUrl: channel.baseUrl,
            status: channel.status,
            apiFormat: channel.apiFormat,
            providerKey: '',
        });
    }

    async function submitCreate(event: FormEvent) {
        event.preventDefault();
        if (
            !draft.channelName.trim() ||
            draft.channelName.length > 20 ||
            draft.baseUrl.length > 100 ||
            saving
        )
            return;
        setSaving(true);
        setError(null);
        try {
            const channel = await createChannel(draft);
            setChannels((current) => [...(current ?? []), channel]);
            setCreating(false);
            setDraft(newDraft());
        } catch (requestError) {
            setError(
                requestError instanceof Error ? requestError.message : 'i18n:channels.createError',
            );
        } finally {
            setSaving(false);
        }
    }

    async function saveEdit(channel: Channel) {
        if (
            !draft.channelName.trim() ||
            draft.channelName.length > 20 ||
            draft.baseUrl.length > 100 ||
            saving
        )
            return;
        setSaving(true);
        setError(null);
        try {
            let updated = channel;
            if (draft.channelName !== updated.channelName)
                updated = await updateChannelName(channel.channelId, draft.channelName);
            if (draft.status !== updated.status)
                updated = await updateChannelStatus(channel.channelId, draft.status);
            if (draft.baseUrl !== updated.baseUrl)
                updated = await updateChannelBaseUrl(channel.channelId, draft.baseUrl);
            if (draft.apiFormat !== updated.apiFormat)
                updated = await updateChannelApiFormat(channel.channelId, draft.apiFormat);
            if (draft.providerKey)
                await updateChannelProviderKey(channel.channelId, draft.providerKey);
            setChannels(
                (current) =>
                    current?.map((item) =>
                        item.channelId === updated.channelId ? updated : item,
                    ) ?? null,
            );
            setDraft((current) => ({ ...current, providerKey: '' }));
            setEditing(null);
        } catch (requestError) {
            setError(
                requestError instanceof Error ? requestError.message : 'i18n:channels.updateError',
            );
        } finally {
            setSaving(false);
        }
    }

    const form = (onSubmit: (event: FormEvent) => void, includeStatus: boolean) => (
        <form onSubmit={onSubmit} className="grid gap-3 md:grid-cols-2">
            <label className="text-sm font-medium text-slate-700">
                {t('channels.name')}
                <input
                    className={inputClass}
                    maxLength={20}
                    value={draft.channelName}
                    onChange={(event) => setDraft({ ...draft, channelName: event.target.value })}
                    required
                />
            </label>
            <label className="text-sm font-medium text-slate-700">
                {t('channels.baseUrl')}
                <input
                    className={inputClass}
                    maxLength={100}
                    value={draft.baseUrl}
                    onChange={(event) => setDraft({ ...draft, baseUrl: event.target.value })}
                    required
                />
            </label>
            {includeStatus && (
                <label className="text-sm font-medium text-slate-700">
                    {t('channels.status')}
                    <select
                        className={inputClass}
                        value={draft.status}
                        onChange={(event) =>
                            setDraft({ ...draft, status: Number(event.target.value) })
                        }
                    >
                        {channelStatuses.map((value) => (
                            <option key={value} value={value}>
                                {channelStatusLabel(value)}
                            </option>
                        ))}
                    </select>
                </label>
            )}
            <label className="text-sm font-medium text-slate-700">
                {t('channels.apiFormat')}
                <select
                    className={inputClass}
                    value={draft.apiFormat}
                    onChange={(event) => {
                        const apiFormat = Number(event.target.value);
                        setDraft({
                            ...draft,
                            apiFormat,
                            baseUrl: defaultBaseUrl(apiFormat),
                        });
                    }}
                >
                    {Object.entries(API_FORMATS).map(([value, label]) => (
                        <option key={value} value={value}>
                            {label}
                        </option>
                    ))}
                </select>
            </label>
            <label className="text-sm font-medium text-slate-700 md:col-span-2">
                {t('channels.providerKey')}
                <input
                    type="password"
                    autoComplete="new-password"
                    className={inputClass}
                    value={draft.providerKey}
                    onChange={(event) => setDraft({ ...draft, providerKey: event.target.value })}
                    placeholder={includeStatus ? t('channels.keepKey') : t('channels.credential')}
                    required={!includeStatus}
                />
            </label>
            <div className="flex gap-2 md:col-span-2">
                <button disabled={saving} className={buttonClass}>
                    {saving ? t('common.saving') : t('common.save')}
                </button>
                <button
                    type="button"
                    onClick={() => {
                        setCreating(false);
                        setEditing(null);
                    }}
                    className="rounded-xl px-4 py-2 text-sm font-semibold text-slate-600"
                >
                    {t('common.cancel')}
                </button>
            </div>
        </form>
    );

    return (
        <div className="mx-auto max-w-7xl space-y-6 p-5 md:p-8">
            <PageHeader
                eyebrow={t('channels.eyebrow')}
                title={t('channels.title')}
                description={t('channels.description')}
                action={
                    <button
                        type="button"
                        onClick={() => {
                            setCreating(true);
                            setEditing(null);
                            setDraft(newDraft());
                        }}
                        className={buttonClass}
                    >
                        <Plus size={16} className="mr-2" /> {t('channels.new')}
                    </button>
                }
            />
            {error && channels && (
                <RefreshWarning message={error} retry={() => setVersion((value) => value + 1)} />
            )}
            {creating && (
                <section className="rounded-2xl border border-emerald-100 bg-white p-5 shadow-sm">
                    <h2 className="mb-4 font-bold text-slate-900">{t('channels.create')}</h2>
                    {form(submitCreate, false)}
                </section>
            )}
            {!channels && !error && <LoadingCards />}
            {!channels && error && (
                <ErrorState message={error} retry={() => setVersion((value) => value + 1)} />
            )}
            {channels?.length === 0 && !creating && (
                <EmptyState
                    title={t('channels.emptyTitle')}
                    description={t('channels.emptyDescription')}
                />
            )}
            <div className="grid gap-4 lg:grid-cols-2">
                {channels?.map((channel) => (
                    <article
                        key={channel.channelId}
                        className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm"
                    >
                        {editing === channel.channelId ? (
                            form((event) => {
                                event.preventDefault();
                                void saveEdit(channel);
                            }, true)
                        ) : (
                            <>
                                <div className="flex items-start justify-between gap-3">
                                    <div>
                                        <h2 className="font-bold text-slate-900">
                                            {channel.channelName}
                                        </h2>
                                        <p className="mt-1 break-all text-sm text-slate-500">
                                            {channel.baseUrl}
                                        </p>
                                    </div>
                                    <button
                                        type="button"
                                        onClick={() => startEdit(channel)}
                                        className="rounded-lg p-2 text-slate-500 hover:bg-slate-100"
                                        aria-label={t('channels.editAria', {
                                            name: channel.channelName,
                                        })}
                                    >
                                        <Pencil size={17} />
                                    </button>
                                </div>
                                <div className="mt-5 flex flex-wrap gap-2 text-xs font-semibold">
                                    <span
                                        className={`rounded-full px-3 py-1 ${channel.status === 1 ? 'bg-emerald-50 text-emerald-700' : channel.status === 2 ? 'bg-rose-50 text-rose-700' : 'bg-amber-50 text-amber-700'}`}
                                    >
                                        {channel.status === 1 ? (
                                            <Check className="mr-1 inline" size={12} />
                                        ) : (
                                            <X className="mr-1 inline" size={12} />
                                        )}
                                        {channelStatusLabel(channel.status)}
                                    </span>
                                    <span className="rounded-full bg-slate-100 px-3 py-1 text-slate-600">
                                        {apiFormatLabel(channel.apiFormat)}
                                    </span>
                                    <span className="rounded-full bg-slate-100 px-3 py-1 text-slate-600">
                                        {t('common.id')} {channel.channelId}
                                    </span>
                                </div>
                            </>
                        )}
                    </article>
                ))}
            </div>
        </div>
    );
}
