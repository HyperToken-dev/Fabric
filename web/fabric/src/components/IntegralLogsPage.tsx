import { useEffect, useState, type FormEvent } from 'react';
import { ArrowLeft, CalendarClock, FileJson, Search } from 'lucide-react';
import { listIntegralLogs, type IntegralLog } from '../api/integralLogs';
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

const pageSize = 50;

function prettyJSON(value: string): string {
    try {
        return JSON.stringify(JSON.parse(value), null, 4);
    } catch {
        return value;
    }
}

function formatJSON(value: string): string | null {
    const trimmed = value.trim();
    if (!trimmed) return null;
    try {
        return JSON.stringify(JSON.parse(trimmed), null, 4);
    } catch {
        return null;
    }
}

function sseChunks(value: string): string[] {
    return value
        .split(/\r?\n\r?\n/)
        .map((chunk) => chunk.trim())
        .filter(Boolean);
}

function textFromValue(value: unknown): string {
    if (typeof value === 'string') return value;
    if (Array.isArray(value)) return value.map(textFromValue).filter(Boolean).join(' ');
    if (!value || typeof value !== 'object') return '';

    const record = value as Record<string, unknown>;
    return [record.text, record.content, record.input].map(textFromValue).filter(Boolean).join(' ');
}

function inputSummaryFromRequest(request: unknown): string {
    if (!request || typeof request !== 'object') return '';
    const record = request as Record<string, unknown>;
    const input =
        textFromValue(record.input) ||
        textFromValue(record.messages) ||
        textFromValue(record.prompt);
    return input.length > 50 ? `${input.slice(0, 50)}...` : input;
}

function logSummary(log: IntegralLog): {
    channelId: string;
    provider: string;
    model: string;
    input: string;
    outcome: string;
    rejectionStage: string;
} {
    try {
        const context = JSON.parse(log.context) as Record<string, unknown>;
        const channelId = typeof context.channel_id === 'number' ? String(context.channel_id) : '-';
        const provider =
            typeof context.provider === 'string' && context.provider.trim()
                ? context.provider
                : '-';
        const model =
            typeof context.model === 'string' && context.model.trim() ? context.model : '-';
        const outcome =
            typeof context.outcome === 'string' && context.outcome.trim() ? context.outcome : 'ok';
        const rejectionStage =
            typeof context.rejection_stage === 'string' && context.rejection_stage.trim()
                ? context.rejection_stage
                : '';
        return {
            channelId,
            provider,
            model,
            input: inputSummaryFromRequest(context.request) || '-',
            outcome,
            rejectionStage,
        };
    } catch {
        return {
            channelId: '-',
            provider: '-',
            model: '-',
            input: '-',
            outcome: 'ok',
            rejectionStage: '',
        };
    }
}

function StatusBadge({ outcome, rejectionStage }: { outcome: string; rejectionStage: string }) {
    const { t } = useI18n();

    if (outcome === 'rejected') {
        return (
            <span className="inline-flex w-fit items-center rounded-full bg-rose-100 px-2.5 py-1 text-xs font-bold uppercase tracking-wide text-rose-700">
                {t('integral.rejected')}
                {rejectionStage ? ` · ${rejectionStage}` : ''}
            </span>
        );
    }
    if (outcome === 'error') {
        return (
            <span className="inline-flex w-fit items-center rounded-full bg-amber-100 px-2.5 py-1 text-xs font-bold uppercase tracking-wide text-amber-700">
                {t('integral.error')}
            </span>
        );
    }
    return (
        <span className="inline-flex w-fit items-center rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-bold uppercase tracking-wide text-emerald-700">
            OK
        </span>
    );
}

function ResponseContent({ response }: { response: string }) {
    const { t } = useI18n();

    if (!response.trim()) {
        return (
            <pre className="max-h-[42vh] overflow-auto whitespace-pre-wrap rounded-xl bg-slate-100 p-4 text-xs leading-relaxed text-slate-700">
                {t('integral.noResponse')}
            </pre>
        );
    }

    const json = formatJSON(response);
    if (json) {
        return (
            <pre className="max-h-[42vh] overflow-auto whitespace-pre-wrap rounded-xl bg-slate-100 p-4 text-xs leading-relaxed text-slate-700">
                {json}
            </pre>
        );
    }

    const chunks = sseChunks(response);
    return (
        <div className="max-h-[42vh] space-y-3 overflow-auto rounded-xl bg-slate-100 p-3">
            {(chunks.length ? chunks : [response]).map((chunk, index) => (
                <div
                    key={`${index}-${chunk.slice(0, 16)}`}
                    className="rounded-lg bg-white shadow-sm"
                >
                    <div className="border-b border-slate-100 px-3 py-2 text-[11px] font-bold uppercase tracking-wider text-slate-400">
                        {t('integral.chunk', { number: index + 1 })}
                    </div>
                    <pre className="whitespace-pre-wrap p-3 text-xs leading-relaxed text-slate-700">
                        {chunk}
                    </pre>
                </div>
            ))}
        </div>
    );
}

export default function IntegralLogsPage() {
    const { t, formatDate } = useI18n();
    const [logs, setLogs] = useState<IntegralLog[] | null>(null);
    const [total, setTotal] = useState(0);
    const [error, setError] = useState<string | null>(null);
    const [version, setVersion] = useState(0);
    const [offset, setOffset] = useState(0);
    const [keyFilter, setKeyFilter] = useState('');
    const [appliedKeyId, setAppliedKeyId] = useState<number | undefined>();
    const [selectedLog, setSelectedLog] = useState<IntegralLog | null>(null);

    useEffect(() => {
        const controller = new AbortController();
        setLogs(null);
        setError(null);
        listIntegralLogs({ keyId: appliedKeyId, limit: pageSize, offset }, controller.signal)
            .then((result) => {
                setLogs(result.logs);
                setTotal(result.total);
                setSelectedLog((current) => {
                    if (!current) return null;
                    return result.logs.find((log) => log.id === current.id) ?? null;
                });
            })
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted) {
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : t('integral.loadError'),
                    );
                }
            });
        return () => controller.abort();
    }, [appliedKeyId, offset, version, t]);

    function applyFilter(event: FormEvent) {
        event.preventDefault();
        const trimmed = keyFilter.trim();
        if (!trimmed) {
            setOffset(0);
            setAppliedKeyId(undefined);
            setSelectedLog(null);
            return;
        }

        const parsed = Number(trimmed);
        if (!Number.isSafeInteger(parsed) || parsed <= 0) {
            setError(t('integral.invalidKeyId'));
            return;
        }
        setOffset(0);
        setAppliedKeyId(parsed);
        setSelectedLog(null);
    }

    const canGoBack = offset > 0;
    const canGoNext = offset + pageSize < total;

    if (!logs && error) {
        return (
            <div className="mx-auto max-w-7xl p-5 md:p-8">
                <ErrorState message={error} retry={() => setVersion((value) => value + 1)} />
            </div>
        );
    }

    if (selectedLog) {
        const summary = logSummary(selectedLog);
        return (
            <div className="mx-auto max-w-5xl space-y-6 p-5 md:p-8">
                <PageHeader
                    eyebrow={t('integral.eyebrow')}
                    title={formatDate(selectedLog.createdAt)}
                    description={t('integral.detail')}
                    action={
                        <button
                            type="button"
                            onClick={() => setSelectedLog(null)}
                            className="inline-flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50"
                        >
                            <ArrowLeft size={16} /> {t('common.back')}
                        </button>
                    }
                />
                {error && (
                    <RefreshWarning
                        message={error}
                        retry={() => setVersion((value) => value + 1)}
                    />
                )}
                <div className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
                    <StatusBadge
                        outcome={summary.outcome}
                        rejectionStage={summary.rejectionStage}
                    />
                </div>
                <section className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm">
                    <div className="mb-3 flex items-center gap-2 text-sm font-bold text-slate-900">
                        <FileJson size={18} className="text-emerald-600" /> {t('integral.context')}
                    </div>
                    <pre className="max-h-[42vh] overflow-auto whitespace-pre-wrap rounded-xl bg-slate-950 p-4 text-xs leading-relaxed text-emerald-50">
                        {prettyJSON(selectedLog.context)}
                    </pre>
                </section>
                <section className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm">
                    <div className="mb-3 flex items-center gap-2 text-sm font-bold text-slate-900">
                        <FileJson size={18} className="text-teal-600" /> {t('integral.response')}
                    </div>
                    <ResponseContent response={selectedLog.response} />
                </section>
            </div>
        );
    }

    return (
        <div className="mx-auto max-w-7xl space-y-6 p-5 md:p-8">
            <PageHeader
                eyebrow={t('integral.eyebrow')}
                title={t('integral.title')}
                description={t('integral.description')}
                action={
                    <form onSubmit={applyFilter} className="flex w-full gap-2 sm:w-auto">
                        <label className="sr-only" htmlFor="integral-key-filter">
                            {t('integral.apiKeyId')}
                        </label>
                        <input
                            id="integral-key-filter"
                            className={`${inputClass} sm:w-44`}
                            inputMode="numeric"
                            value={keyFilter}
                            onChange={(event) => setKeyFilter(event.target.value)}
                            placeholder="key_id"
                        />
                        <button className={buttonClass} aria-label={t('integral.applyFilter')}>
                            <Search size={16} />
                        </button>
                    </form>
                }
            />
            {error && logs && (
                <RefreshWarning message={error} retry={() => setVersion((value) => value + 1)} />
            )}
            {!logs && <LoadingCards />}
            {logs?.length === 0 && (
                <EmptyState
                    title={t('integral.emptyTitle')}
                    description={t('integral.emptyDescription')}
                />
            )}
            {!!logs?.length && (
                <section className="space-y-4">
                    <div className="flex flex-col gap-3 rounded-2xl border border-slate-100 bg-white px-5 py-4 text-sm text-slate-500 shadow-sm sm:flex-row sm:items-center sm:justify-between">
                        <span>
                            {t('integral.showing', {
                                start: offset + 1,
                                end: Math.min(offset + logs.length, total),
                                total,
                            })}
                        </span>
                        <div className="flex gap-2">
                            <button
                                type="button"
                                disabled={!canGoBack}
                                onClick={() => setOffset((value) => Math.max(0, value - pageSize))}
                                className="rounded-xl border border-slate-200 px-3 py-2 font-semibold text-slate-700 disabled:opacity-50"
                            >
                                {t('common.previous')}
                            </button>
                            <button
                                type="button"
                                disabled={!canGoNext}
                                onClick={() => setOffset((value) => value + pageSize)}
                                className="rounded-xl border border-slate-200 px-3 py-2 font-semibold text-slate-700 disabled:opacity-50"
                            >
                                {t('common.next')}
                            </button>
                        </div>
                    </div>
                    <div className="overflow-hidden rounded-2xl border border-slate-100 bg-white shadow-sm">
                        {logs.map((log) => (
                            <LogListItem
                                key={log.id}
                                log={log}
                                onClick={() => setSelectedLog(log)}
                            />
                        ))}
                    </div>
                </section>
            )}
        </div>
    );
}

function LogListItem({ log, onClick }: { log: IntegralLog; onClick: () => void }) {
    const { t, formatDate } = useI18n();
    const summary = logSummary(log);

    return (
        <button
            type="button"
            onClick={onClick}
            className="grid w-full gap-3 border-b border-slate-100 px-5 py-4 text-left transition last:border-b-0 hover:bg-emerald-50/60 md:grid-cols-[220px_110px_130px_minmax(160px,1fr)_minmax(180px,1.4fr)] md:items-center"
        >
            <span className="flex items-center gap-3 font-semibold text-slate-900">
                <span className="rounded-xl bg-emerald-50 p-2.5 text-emerald-700">
                    <CalendarClock size={18} />
                </span>
                {formatDate(log.createdAt)}
            </span>
            <span className="text-sm text-slate-600">
                {t('integral.keyId', { value: log.keyId })}
            </span>
            <span className="text-sm text-slate-600">
                {t('integral.channelId', { value: summary.channelId })}
            </span>
            <div className="flex flex-col gap-1">
                <span className="truncate text-sm font-medium text-slate-700" title={summary.model}>
                    {summary.model}
                </span>
                <span className="text-xs font-semibold uppercase tracking-wide text-slate-400">
                    {summary.provider}
                </span>
            </div>
            <div className="space-y-2">
                <StatusBadge outcome={summary.outcome} rejectionStage={summary.rejectionStage} />
                <span className="block truncate text-sm text-slate-500" title={summary.input}>
                    {summary.input}
                </span>
            </div>
        </button>
    );
}
