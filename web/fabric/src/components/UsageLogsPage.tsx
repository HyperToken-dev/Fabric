import { useEffect, useState, type FormEvent } from 'react';
import { Activity, Cpu, KeyRound, Search, Zap } from 'lucide-react';
import { listAdminApiKeys, type ApiKey } from '../api/apiKeys';
import { listChannels, type Channel } from '../api/channels';
import { listModels, type Model } from '../api/models';
import {
    getUsageByChannelId,
    getUsageByDeadlineAndKeyHash,
    getUsageByKeyHash,
    getUsageByModelId,
    getUsageSummary,
    type UsageLog,
    type UsageSummary,
} from '../api/usage';
import {
    buttonClass,
    EmptyState,
    ErrorState,
    inputClass,
    PageHeader,
    RefreshWarning,
} from './PageState';
import { useI18n } from '../i18n';

type QueryMode = 'channel' | 'model' | 'key' | 'deadline';
type DeadlinePreset = '1' | '7' | '30' | '90';

export default function UsageLogsPage() {
    const { t, formatNumber, formatDate } = useI18n();
    const [summary, setSummary] = useState<UsageSummary | null>(null);
    const [summaryError, setSummaryError] = useState<string | null>(null);
    const [summaryVersion, setSummaryVersion] = useState(0);
    const [channels, setChannels] = useState<Channel[] | null>(null);
    const [channelsError, setChannelsError] = useState<string | null>(null);
    const [channelId, setChannelId] = useState<number | null>(null);
    const [models, setModels] = useState<Model[]>([]);
    const [modelId, setModelId] = useState<number | null>(null);
    const [keys, setKeys] = useState<ApiKey[]>([]);
    const [keyHash, setKeyHash] = useState('');
    const [optionsLoading, setOptionsLoading] = useState(false);
    const [mode, setMode] = useState<QueryMode>('channel');
    const [deadlinePreset, setDeadlinePreset] = useState<DeadlinePreset>('7');
    const [logs, setLogs] = useState<UsageLog[] | null>(null);
    const [queryError, setQueryError] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);
    const [activeFilter, setActiveFilter] = useState('');

    useEffect(() => {
        const controller = new AbortController();
        setSummaryError(null);
        getUsageSummary(controller.signal)
            .then(setSummary)
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted)
                    setSummaryError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'i18n:usage.summaryError',
                    );
            });
        return () => controller.abort();
    }, [summaryVersion]);

    useEffect(() => {
        const controller = new AbortController();
        setChannelsError(null);
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
                    setChannelsError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'i18n:usage.channelsError',
                    );
            });
        return () => controller.abort();
    }, [summaryVersion]);

    useEffect(() => {
        setLogs(null);
        setQueryError(null);
        if (!channelId || mode === 'channel') {
            setModels([]);
            setKeys([]);
            return;
        }
        const controller = new AbortController();
        setOptionsLoading(true);
        const request =
            mode === 'model'
                ? listModels(channelId, controller.signal)
                : listAdminApiKeys(controller.signal).then((items) =>
                      items.filter((item) => item.channelId === channelId),
                  );
        request
            .then((result) => {
                if (mode === 'model') {
                    const nextModels = result as Model[];
                    setModels(nextModels);
                    setModelId(nextModels[0]?.modelId ?? null);
                } else {
                    const nextKeys = result as ApiKey[];
                    setKeys(nextKeys);
                    setKeyHash(nextKeys[0]?.keyHash ?? '');
                }
            })
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted)
                    setQueryError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'i18n:usage.optionsError',
                    );
            })
            .finally(() => {
                if (!controller.signal.aborted) setOptionsLoading(false);
            });
        return () => controller.abort();
    }, [channelId, mode]);

    async function submit(event: FormEvent) {
        event.preventDefault();
        if (
            !channelId ||
            loading ||
            (mode === 'model' && !modelId) ||
            ((mode === 'key' || mode === 'deadline') && !keyHash)
        )
            return;
        setLoading(true);
        setQueryError(null);
        try {
            const channel = channels?.find((item) => item.channelId === channelId);
            let result: UsageLog[];
            let filter = t('usage.filterChannel', { value: channel?.channelName ?? channelId });
            if (mode === 'channel') {
                result = await getUsageByChannelId(channelId);
            } else if (mode === 'model' && modelId) {
                const model = models.find((item) => item.modelId === modelId);
                result = await getUsageByModelId(modelId);
                filter = t('usage.filterModel', {
                    model: model?.modelName ?? modelId,
                    channel: channel?.channelName ?? channelId,
                });
            } else if (mode === 'key') {
                const key = keys.find((item) => item.keyHash === keyHash);
                result = await getUsageByKeyHash(keyHash);
                filter = t('usage.filterKey', {
                    key: key?.keyName ?? t('usage.selectedKey'),
                    channel: channel?.channelName ?? channelId,
                });
            } else {
                const days = Number(deadlinePreset);
                const deadline = new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString();
                const key = keys.find((item) => item.keyHash === keyHash);
                result = await getUsageByDeadlineAndKeyHash(keyHash, deadline);
                filter = t('usage.filterDeadline', {
                    key: key?.keyName ?? t('usage.selectedKey'),
                    days,
                    plural: days === 1 ? '' : 's',
                });
            }
            setLogs(result);
            setActiveFilter(filter);
        } catch (requestError) {
            setQueryError(
                requestError instanceof Error ? requestError.message : 'i18n:usage.queryError',
            );
        } finally {
            setLoading(false);
        }
    }

    const stats = summary
        ? [
              {
                  label: t('usage.promptTokens'),
                  value: summary.promptTokens,
                  icon: Cpu,
                  color: 'text-emerald-600',
                  surface: 'bg-emerald-50',
              },
              {
                  label: t('usage.completionTokens'),
                  value: summary.completionTokens,
                  icon: Zap,
                  color: 'text-amber-600',
                  surface: 'bg-amber-50',
              },
              {
                  label: t('usage.totalTokens'),
                  value: summary.totalTokens,
                  icon: Activity,
                  color: 'text-teal-700',
                  surface: 'bg-teal-50',
              },
          ]
        : [];
    const canSearch =
        !!channelId &&
        !optionsLoading &&
        (mode === 'channel' || (mode === 'model' ? !!modelId : !!keyHash));

    return (
        <div className="mx-auto max-w-7xl space-y-6 p-5 md:p-8">
            <PageHeader
                eyebrow={t('usage.eyebrow')}
                title={t('usage.title')}
                description={t('usage.description')}
            />
            {summaryError && summary && (
                <RefreshWarning
                    message={summaryError}
                    retry={() => setSummaryVersion((value) => value + 1)}
                />
            )}
            {!summary && !summaryError && (
                <div className="h-32 animate-pulse rounded-2xl bg-white" />
            )}
            {!summary && summaryError && (
                <ErrorState
                    message={summaryError}
                    retry={() => setSummaryVersion((value) => value + 1)}
                />
            )}
            {!!summary && (
                <div className="grid gap-4 sm:grid-cols-3">
                    {stats.map(({ label, value, icon: Icon, color, surface }) => (
                        <article
                            key={label}
                            className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm"
                        >
                            <div className={`inline-flex rounded-xl p-2.5 ${surface}`}>
                                <Icon className={color} size={20} />
                            </div>
                            <p className="mt-4 text-sm text-slate-500">{label}</p>
                            <p className="mt-1 text-2xl font-bold text-slate-900">
                                {formatNumber(value)}
                            </p>
                        </article>
                    ))}
                </div>
            )}
            {channelsError && (
                <ErrorState
                    message={channelsError}
                    retry={() => setSummaryVersion((value) => value + 1)}
                />
            )}
            {channels?.length === 0 && (
                <EmptyState
                    title={t('usage.noChannelsTitle')}
                    description={t('usage.noChannelsDescription')}
                />
            )}
            {!!channels?.length && (
                <section className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm">
                    <div className="mb-5 flex items-center gap-3">
                        <div className="rounded-xl bg-emerald-50 p-2.5 text-emerald-700">
                            <Search size={19} />
                        </div>
                        <div>
                            <h2 className="font-bold text-slate-900">{t('usage.explore')}</h2>
                            <p className="text-sm text-slate-500">
                                {t('usage.exploreDescription')}
                            </p>
                        </div>
                    </div>
                    <form
                        onSubmit={submit}
                        className="grid gap-3 lg:grid-cols-[180px_1fr_1fr_180px_auto] lg:items-end"
                    >
                        <label className="text-sm font-medium text-slate-700">
                            {t('usage.viewBy')}
                            <select
                                className={inputClass}
                                value={mode}
                                onChange={(event) => setMode(event.target.value as QueryMode)}
                            >
                                <option value="channel">{t('usage.byChannel')}</option>
                                <option value="model">{t('usage.byModel')}</option>
                                <option value="key">{t('usage.byKey')}</option>
                                <option value="deadline">{t('usage.byDeadline')}</option>
                            </select>
                        </label>
                        <label className="text-sm font-medium text-slate-700">
                            {t('usage.channel')}
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
                        {mode === 'model' && (
                            <label className="text-sm font-medium text-slate-700">
                                {t('usage.model')}
                                <select
                                    className={inputClass}
                                    disabled={optionsLoading || models.length === 0}
                                    value={modelId ?? ''}
                                    onChange={(event) => setModelId(Number(event.target.value))}
                                >
                                    {models.map((model) => (
                                        <option key={model.modelId} value={model.modelId}>
                                            {model.modelName}
                                        </option>
                                    ))}
                                </select>
                            </label>
                        )}
                        {(mode === 'key' || mode === 'deadline') && (
                            <label className="text-sm font-medium text-slate-700">
                                {t('usage.apiKey')}
                                <select
                                    className={inputClass}
                                    disabled={optionsLoading || keys.length === 0}
                                    value={keyHash}
                                    onChange={(event) => setKeyHash(event.target.value)}
                                >
                                    {keys.map((key) => (
                                        <option key={key.keyHash} value={key.keyHash}>
                                            {key.keyName}
                                        </option>
                                    ))}
                                </select>
                            </label>
                        )}
                        {mode === 'channel' && <div />}
                        {mode === 'deadline' ? (
                            <label className="text-sm font-medium text-slate-700">
                                {t('usage.period')}
                                <select
                                    className={inputClass}
                                    value={deadlinePreset}
                                    onChange={(event) =>
                                        setDeadlinePreset(event.target.value as DeadlinePreset)
                                    }
                                >
                                    <option value="1">{t('usage.last24Hours')}</option>
                                    <option value="7">{t('usage.lastDays', { days: 7 })}</option>
                                    <option value="30">{t('usage.lastDays', { days: 30 })}</option>
                                    <option value="90">{t('usage.lastDays', { days: 90 })}</option>
                                </select>
                            </label>
                        ) : (
                            <div />
                        )}
                        <button disabled={!canSearch || loading} className={buttonClass}>
                            <Search size={16} className="mr-2" />
                            {loading ? t('common.loading') : t('usage.showUsage')}
                        </button>
                    </form>
                    {optionsLoading && (
                        <p className="mt-3 text-xs text-slate-500">{t('usage.loadingResources')}</p>
                    )}
                    {!optionsLoading && mode === 'model' && models.length === 0 && (
                        <p className="mt-3 text-xs font-medium text-amber-700">
                            {t('usage.noModels')}
                        </p>
                    )}
                    {!optionsLoading &&
                        (mode === 'key' || mode === 'deadline') &&
                        keys.length === 0 && (
                            <p className="mt-3 text-xs font-medium text-amber-700">
                                {t('usage.noKeys')}
                            </p>
                        )}
                </section>
            )}
            {queryError && logs && (
                <RefreshWarning message={queryError} retry={() => setQueryError(null)} />
            )}
            {queryError && !logs && (
                <ErrorState message={queryError} retry={() => setQueryError(null)} />
            )}
            {loading && !logs && <div className="h-48 animate-pulse rounded-2xl bg-white" />}
            {logs?.length === 0 && (
                <EmptyState
                    title={t('usage.noMatchingTitle')}
                    description={t('usage.noMatchingDescription', { filter: activeFilter })}
                />
            )}
            {!!logs?.length && (
                <section className="overflow-hidden rounded-2xl border border-slate-100 bg-white shadow-sm">
                    <div className="flex items-center justify-between border-b border-slate-100 px-5 py-4">
                        <div>
                            <h2 className="font-bold text-slate-900">{t('usage.results')}</h2>
                            <p className="text-xs text-slate-500">{activeFilter}</p>
                        </div>
                        <span className="rounded-full bg-slate-100 px-3 py-1 text-xs font-semibold text-slate-600">
                            {t('common.records', { count: logs.length })}
                        </span>
                    </div>
                    <div className="overflow-x-auto">
                        <table className="w-full text-left text-sm">
                            <thead className="bg-slate-50 text-xs uppercase text-slate-500">
                                <tr>
                                    <th className="px-5 py-3">{t('usage.created')}</th>
                                    <th className="px-5 py-3">{t('usage.channel')}</th>
                                    <th className="px-5 py-3">{t('usage.model')}</th>
                                    <th className="px-5 py-3">{t('usage.key')}</th>
                                    <th className="px-5 py-3">{t('common.ownerOpenid')}</th>
                                    <th className="px-5 py-3">{t('dashboard.prompt')}</th>
                                    <th className="px-5 py-3">{t('dashboard.completion')}</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100">
                                {logs.map((log, index) => (
                                    <tr key={log.usageId || `${log.modelId}-${index}`}>
                                        <td className="whitespace-nowrap px-5 py-4 text-slate-500">
                                            {log.createdAt
                                                ? formatDate(log.createdAt)
                                                : t('common.aggregated')}
                                        </td>
                                        <td className="px-5 py-4">{log.channelId || '-'}</td>
                                        <td className="px-5 py-4">{log.modelId || '-'}</td>
                                        <td className="px-5 py-4">
                                            <KeyRound size={14} className="text-slate-400" />
                                        </td>
                                        <td
                                            className="max-w-[220px] truncate px-5 py-4 text-xs text-slate-500"
                                            title={log.ownerOpenid}
                                        >
                                            {log.ownerOpenid || '-'}
                                        </td>
                                        <td className="px-5 py-4 font-medium">
                                            {formatNumber(log.promptTokens)}
                                        </td>
                                        <td className="px-5 py-4 font-medium">
                                            {formatNumber(log.completionTokens)}
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </section>
            )}
        </div>
    );
}
