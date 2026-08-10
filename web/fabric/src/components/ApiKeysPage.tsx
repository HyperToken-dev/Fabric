import { useEffect, useState, type FormEvent } from 'react';
import { Copy, KeyRound, Plus, Trash2, X } from 'lucide-react';
import {
    createApiKey,
    listApiKeys,
    revokeApiKey,
    type ApiKey,
    type CreatedApiKey,
} from '../api/apiKeys';
import { listChannels, type Channel } from '../api/channels';
import {
    buttonClass,
    EmptyState,
    ErrorState,
    inputClass,
    LoadingCards,
    PageHeader,
    RefreshWarning,
} from './PageState';
import { useI18n } from '../i18n';

export default function ApiKeysPage() {
    const { t, formatDate } = useI18n();
    const [channels, setChannels] = useState<Channel[] | null>(null);
    const [channelId, setChannelId] = useState<number | null>(null);
    const [keys, setKeys] = useState<ApiKey[] | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [version, setVersion] = useState(0);
    const [keyName, setKeyName] = useState('');
    const [saving, setSaving] = useState(false);
    const [revoking, setRevoking] = useState<string | null>(null);
    const [created, setCreated] = useState<CreatedApiKey | null>(null);
    const [copied, setCopied] = useState(false);
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
                            : t('apiKeys.loadChannelsError'),
                    );
            });
        return () => controller.abort();
    }, [version, t]);
    useEffect(() => {
        if (!channelId) {
            setKeys([]);
            return;
        }
        const controller = new AbortController();
        setKeys(null);
        setError(null);
        listApiKeys(channelId, controller.signal)
            .then(setKeys)
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted)
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : t('apiKeys.loadError'),
                    );
            });
        return () => controller.abort();
    }, [channelId, version, t]);
    async function submit(event: FormEvent) {
        event.preventDefault();
        if (!channelId || !keyName.trim() || saving) return;
        setSaving(true);
        setError(null);
        try {
            const key = await createApiKey(keyName.trim(), channelId);
            setCreated(key);
            setCopied(false);
            setKeyName('');
            setKeys((current) => [
                ...(current ?? []),
                { keyName: key.keyName, keyHash: key.keyHash, createdAt: key.createdAt },
            ]);
        } catch (requestError) {
            setError(
                requestError instanceof Error ? requestError.message : t('apiKeys.createError'),
            );
        } finally {
            setSaving(false);
        }
    }
    async function revoke(key: ApiKey) {
        if (revoking || !window.confirm(t('apiKeys.revokeConfirm', { name: key.keyName }))) return;
        setRevoking(key.keyHash);
        setError(null);
        try {
            await revokeApiKey(key.keyHash);
            setKeys((current) => current?.filter((item) => item.keyHash !== key.keyHash) ?? null);
        } catch (requestError) {
            setError(
                requestError instanceof Error ? requestError.message : t('apiKeys.revokeError'),
            );
        } finally {
            setRevoking(null);
        }
    }
    async function copyRawKey() {
        if (!created) return;
        try {
            await navigator.clipboard.writeText(created.rawKey);
            setCopied(true);
        } catch {
            setError(t('apiKeys.clipboardDenied'));
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
                eyebrow={t('apiKeys.eyebrow')}
                title={t('apiKeys.title')}
                description={t('apiKeys.description')}
            />
            {error && channels && (
                <RefreshWarning message={error} retry={() => setVersion((value) => value + 1)} />
            )}
            {!channels && <LoadingCards />}
            {channels?.length === 0 && (
                <EmptyState
                    title={t('apiKeys.createChannelFirst')}
                    description={t('apiKeys.channelFirstDescription')}
                />
            )}
            {!!channels?.length && (
                <>
                    <section className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm">
                        <form
                            onSubmit={submit}
                            className="grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end"
                        >
                            <label className="text-sm font-medium text-slate-700">
                                {t('apiKeys.channel')}
                                <select
                                    className={inputClass}
                                    value={channelId ?? ''}
                                    onChange={(event) => setChannelId(Number(event.target.value))}
                                >
                                    {channels.map((channel) => (
                                        <option key={channel.channelId} value={channel.channelId}>
                                            {channel.channelName}
                                        </option>
                                    ))}
                                </select>
                            </label>
                            <label className="text-sm font-medium text-slate-700">
                                {t('apiKeys.keyName')}
                                <input
                                    className={inputClass}
                                    value={keyName}
                                    onChange={(event) => setKeyName(event.target.value)}
                                    required
                                />
                            </label>
                            <button disabled={saving} className={buttonClass}>
                                <Plus size={16} className="mr-2" />
                                {saving ? t('common.creating') : t('apiKeys.createKey')}
                            </button>
                        </form>
                    </section>
                    {!keys && <LoadingCards />}
                    {keys?.length === 0 && (
                        <EmptyState
                            title={t('apiKeys.emptyTitle')}
                            description={t('apiKeys.emptyDescription')}
                        />
                    )}
                    {!!keys?.length && (
                        <div className="grid gap-4">
                            {keys.map((key) => (
                                <article
                                    key={key.keyHash}
                                    className="flex flex-col gap-4 rounded-2xl border border-slate-100 bg-white p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between"
                                >
                                    <div className="min-w-0">
                                        <div className="flex items-center gap-2">
                                            <KeyRound size={17} className="text-emerald-600" />
                                            <h2 className="font-bold text-slate-900">
                                                {key.keyName}
                                            </h2>
                                        </div>
                                        <p
                                            className="mt-2 truncate font-mono text-xs text-slate-500"
                                            title={key.keyHash}
                                        >
                                            {key.keyHash}
                                        </p>
                                        <p className="mt-1 text-xs text-slate-400">
                                            {t('common.created', {
                                                date: formatDate(key.createdAt),
                                            })}
                                        </p>
                                    </div>
                                    <button
                                        type="button"
                                        disabled={revoking === key.keyHash}
                                        onClick={() => void revoke(key)}
                                        className="inline-flex items-center justify-center gap-2 rounded-xl border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-600 hover:bg-rose-50 disabled:opacity-50"
                                    >
                                        <Trash2 size={15} />
                                        {revoking === key.keyHash
                                            ? t('apiKeys.revoking')
                                            : t('apiKeys.revoke')}
                                    </button>
                                </article>
                            ))}
                        </div>
                    )}
                </>
            )}
            {created && (
                <div
                    className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 p-4"
                    role="dialog"
                    aria-modal="true"
                    aria-labelledby="created-key-title"
                >
                    <div className="w-full max-w-xl rounded-2xl bg-white p-6 shadow-2xl">
                        <div className="flex items-start justify-between">
                            <div>
                                <p className="text-xs font-semibold uppercase tracking-wider text-amber-600">
                                    {t('apiKeys.shownOnce')}
                                </p>
                                <h2
                                    id="created-key-title"
                                    className="mt-1 text-xl font-bold text-slate-900"
                                >
                                    {t('apiKeys.storeNow')}
                                </h2>
                            </div>
                            <button
                                type="button"
                                aria-label={t('apiKeys.close')}
                                onClick={() => setCreated(null)}
                                className="rounded-lg p-2 text-slate-500 hover:bg-slate-100"
                            >
                                <X size={18} />
                            </button>
                        </div>
                        <p className="mt-3 text-sm text-slate-600">{t('apiKeys.rawWarning')}</p>
                        <div className="mt-4 flex items-center gap-2 rounded-xl bg-slate-950 p-3 text-white">
                            <code className="min-w-0 flex-1 break-all text-xs">
                                {created.rawKey}
                            </code>
                            <button
                                type="button"
                                onClick={() => void copyRawKey()}
                                className="shrink-0 rounded-lg bg-white/10 p-2"
                                aria-label={t('apiKeys.copy')}
                            >
                                <Copy size={16} />
                            </button>
                        </div>
                        <p className="mt-2 text-xs font-medium text-emerald-600" aria-live="polite">
                            {copied ? t('apiKeys.copied') : ''}
                        </p>
                        <button
                            type="button"
                            onClick={() => setCreated(null)}
                            className={`${buttonClass} mt-5 w-full`}
                        >
                            {t('apiKeys.stored')}
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
