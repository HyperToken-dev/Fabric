import { useEffect, useState } from 'react';
import {
    Area,
    AreaChart,
    CartesianGrid,
    Legend,
    Line,
    ResponsiveContainer,
    Tooltip,
    XAxis,
    YAxis,
} from 'recharts';
import {
    Activity,
    ArrowUpRight,
    CalendarDays,
    Cpu,
    Gauge,
    RefreshCw,
    Send,
    Sparkles,
    Zap,
} from 'lucide-react';
import { motion } from 'motion/react';
import { getUsageDashboard, type UsageDashboard } from '../api/dashboard';
import { useI18n } from '../i18n';

export default function Dashboard() {
    const { t, formatNumber, formatCompactNumber, formatErrorMessage } = useI18n();
    const [dashboard, setDashboard] = useState<UsageDashboard | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [requestVersion, setRequestVersion] = useState(0);

    useEffect(() => {
        const controller = new AbortController();
        setError(null);
        getUsageDashboard(controller.signal)
            .then(setDashboard)
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted) {
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : t('dashboard.loadError'),
                    );
                }
            });
        return () => controller.abort();
    }, [requestVersion, t]);

    if (!dashboard && !error) {
        return (
            <div
                className="mx-auto max-w-7xl space-y-6 p-5 animate-pulse md:p-8"
                aria-label={t('dashboard.loading')}
            >
                <div className="h-56 rounded-3xl bg-slate-900" />
                <div className="grid gap-4 md:grid-cols-3">
                    {[0, 1, 2].map((item) => (
                        <div key={item} className="h-32 rounded-2xl bg-white" />
                    ))}
                </div>
                <div className="h-[420px] rounded-2xl bg-white" />
            </div>
        );
    }

    if (!dashboard && error) {
        return (
            <div className="flex h-full items-center justify-center p-8">
                <div className="max-w-md rounded-2xl border border-rose-100 bg-white p-8 text-center shadow-sm">
                    <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-rose-50 text-rose-600">
                        <Activity size={24} />
                    </div>
                    <h1 className="text-xl font-bold text-slate-900">
                        {t('dashboard.unavailable')}
                    </h1>
                    <p className="mt-2 text-sm text-slate-500">{formatErrorMessage(error)}</p>
                    <button
                        type="button"
                        onClick={() => setRequestVersion((version) => version + 1)}
                        className="mt-6 inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2.5 text-sm font-semibold text-white hover:bg-slate-800"
                    >
                        <RefreshCw size={16} /> {t('common.retry')}
                    </button>
                </div>
            </div>
        );
    }

    if (!dashboard) return null;

    const sevenDay = dashboard.recentDays.reduce(
        (total, day) => ({
            tokens: total.tokens + day.totalTokens,
            requests: total.requests + day.requestCount,
        }),
        { tokens: 0, requests: 0 },
    );
    const peakDay = dashboard.recentDays.reduce<(typeof dashboard.recentDays)[number] | null>(
        (peak, day) => (!peak || day.totalTokens > peak.totalTokens ? day : peak),
        null,
    );
    const completionShare = dashboard.today.totalTokens
        ? Math.round((dashboard.today.completionTokens / dashboard.today.totalTokens) * 100)
        : 0;
    const dailyAverage = Math.round(sevenDay.tokens / Math.max(dashboard.recentDays.length, 1));
    const todayVsAverage = dailyAverage
        ? Math.round(((dashboard.today.totalTokens - dailyAverage) / dailyAverage) * 100)
        : 0;
    const hasUsage = sevenDay.tokens > 0;
    const supportingStats = [
        {
            title: t('dashboard.volume'),
            value: sevenDay.tokens,
            detail: t('dashboard.requestCount', { count: formatNumber(sevenDay.requests) }),
            icon: CalendarDays,
            color: 'text-emerald-700',
            bg: 'bg-emerald-50',
        },
        {
            title: t('dashboard.dailyAverage'),
            value: dailyAverage,
            detail: t('dashboard.requestsPerDay', {
                count: formatNumber(
                    Math.round(sevenDay.requests / Math.max(dashboard.recentDays.length, 1)),
                ),
            }),
            icon: Gauge,
            color: 'text-sky-600',
            bg: 'bg-sky-50',
        },
        {
            title: t('dashboard.peakDay'),
            value: peakDay?.totalTokens ?? 0,
            detail: peakDay?.date ?? t('dashboard.noActivity'),
            icon: Sparkles,
            color: 'text-amber-600',
            bg: 'bg-amber-50',
        },
    ];

    return (
        <motion.div
            initial={{ opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.35 }}
            className="mx-auto max-w-7xl space-y-6 p-5 md:p-8"
        >
            <section className="relative overflow-hidden rounded-3xl border border-emerald-100 bg-gradient-to-br from-emerald-50 via-teal-50 to-white px-6 py-7 text-slate-900 shadow-xl shadow-emerald-100/60 md:px-8 md:py-9">
                <div className="absolute -right-20 -top-28 h-72 w-72 rounded-full bg-emerald-200/50 blur-3xl" />
                <div className="absolute -bottom-32 left-1/3 h-64 w-64 rounded-full bg-teal-200/40 blur-3xl" />
                <div className="relative grid gap-8 lg:grid-cols-[1.35fr_1fr] lg:items-end">
                    <div>
                        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.2em] text-emerald-700">
                            <Activity size={14} /> {t('dashboard.livePulse')}
                        </div>
                        <div
                            className="relative mt-3 h-10 max-w-xl overflow-hidden"
                            aria-hidden="true"
                        >
                            <div className="absolute inset-x-0 top-1/2 h-px bg-emerald-200" />
                            <motion.svg
                                viewBox="0 0 620 40"
                                preserveAspectRatio="none"
                                className="absolute inset-0 h-full w-full overflow-visible"
                            >
                                <motion.path
                                    d="M0 22 H72 L84 22 L92 16 L101 31 L112 5 L124 35 L136 22 H214 L224 22 L232 18 L240 27 L250 11 L260 31 L272 22 H356 L366 22 L374 15 L384 32 L395 4 L407 35 L420 22 H500 L510 22 L518 17 L527 29 L537 9 L548 33 L560 22 H620"
                                    fill="none"
                                    stroke="#059669"
                                    strokeWidth="2.5"
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    initial={{ pathLength: 0, opacity: 0 }}
                                    animate={{ pathLength: [0, 1, 1], opacity: [0.2, 1, 0.15] }}
                                    transition={{
                                        duration: 1.8,
                                        times: [0, 0.82, 1],
                                        ease: 'linear',
                                        repeat: Infinity,
                                        repeatDelay: 0.08,
                                    }}
                                />
                            </motion.svg>
                            <motion.div
                                className="absolute top-1/2 h-9 w-12 -translate-y-1/2 bg-gradient-to-r from-transparent via-emerald-200/60 to-transparent blur-sm"
                                animate={{ left: ['-12%', '105%'] }}
                                transition={{
                                    duration: 1.8,
                                    ease: 'linear',
                                    repeat: Infinity,
                                    repeatDelay: 0.08,
                                }}
                            />
                        </div>
                        <h1 className="mt-2 text-4xl font-bold tracking-tight md:text-5xl">
                            {formatCompactNumber(dashboard.today.totalTokens)}{' '}
                            <span className="text-2xl font-medium text-emerald-800/55 md:text-3xl">
                                {t('dashboard.tokensToday')}
                            </span>
                        </h1>
                        <p className="mt-3 max-w-xl text-sm leading-6 text-slate-600">
                            {t('dashboard.description')}
                        </p>
                        <div className="mt-6 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-white/70 px-3 py-1.5 text-xs text-emerald-800 shadow-sm">
                            <span className="h-2 w-2 rounded-full bg-emerald-500 shadow-[0_0_10px_#34d399]" />{' '}
                            {t('dashboard.reporting', { timeZone: dashboard.timeZone })}
                        </div>
                    </div>
                    <div className="grid grid-cols-3 gap-3">
                        <div className="rounded-2xl border border-emerald-100 bg-white/70 p-4 shadow-sm backdrop-blur">
                            <Cpu size={17} className="text-emerald-600" />
                            <p className="mt-4 text-xl font-bold text-slate-900">
                                {formatCompactNumber(dashboard.today.promptTokens)}
                            </p>
                            <p className="mt-1 text-xs text-slate-500">{t('dashboard.prompt')}</p>
                        </div>
                        <div className="rounded-2xl border border-lime-100 bg-white/70 p-4 shadow-sm backdrop-blur">
                            <Zap size={17} className="text-lime-600" />
                            <p className="mt-4 text-xl font-bold text-slate-900">
                                {formatCompactNumber(dashboard.today.completionTokens)}
                            </p>
                            <p className="mt-1 text-xs text-slate-500">
                                {t('dashboard.completion')}
                            </p>
                        </div>
                        <div className="rounded-2xl border border-teal-100 bg-white/70 p-4 shadow-sm backdrop-blur">
                            <Send size={17} className="text-teal-600" />
                            <p className="mt-4 text-xl font-bold text-slate-900">
                                {formatCompactNumber(dashboard.today.requestCount)}
                            </p>
                            <p className="mt-1 text-xs text-slate-500">{t('dashboard.requests')}</p>
                        </div>
                    </div>
                </div>
            </section>

            {error && (
                <div className="flex items-center justify-between rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                    <span>{t('common.refreshFailed', { message: formatErrorMessage(error) })}</span>
                    <button
                        type="button"
                        onClick={() => setRequestVersion((version) => version + 1)}
                        className="font-semibold underline"
                    >
                        {t('common.retry')}
                    </button>
                </div>
            )}

            <div className="grid gap-4 md:grid-cols-3">
                {supportingStats.map((stat) => {
                    const Icon = stat.icon;
                    return (
                        <article
                            key={stat.title}
                            className="group rounded-2xl border border-slate-100 bg-white p-5 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md"
                        >
                            <div className="flex items-start justify-between">
                                <div className={`rounded-xl p-2.5 ${stat.bg}`}>
                                    <Icon className={stat.color} size={19} />
                                </div>
                                <ArrowUpRight
                                    size={17}
                                    className="text-slate-300 transition group-hover:text-emerald-500"
                                />
                            </div>
                            <p className="mt-5 text-sm font-medium text-slate-500">{stat.title}</p>
                            <p className="mt-1 text-2xl font-bold tracking-tight text-slate-900">
                                {formatNumber(stat.value)}
                            </p>
                            <p className="mt-1 text-xs text-slate-400">{stat.detail}</p>
                        </article>
                    );
                })}
            </div>

            <div className="grid gap-6 xl:grid-cols-[1fr_300px]">
                <section className="rounded-2xl border border-emerald-100/70 bg-white p-5 shadow-sm md:p-6">
                    <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div>
                            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-emerald-700">
                                {t('dashboard.trafficShape')}
                            </p>
                            <h2 className="mt-1 text-lg font-bold text-slate-900">
                                {t('dashboard.tokenFlow')}
                            </h2>
                            <p className="text-sm text-slate-500">{t('dashboard.activity')}</p>
                        </div>
                        <div
                            className={`inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold ${todayVsAverage >= 0 ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'}`}
                        >
                            {todayVsAverage >= 0 ? '+' : ''}
                            {t('dashboard.vsAverage', {
                                value: `${todayVsAverage >= 0 ? '+' : ''}${todayVsAverage}`,
                            })}
                        </div>
                    </div>
                    <div className="h-[350px] w-full">
                        <ResponsiveContainer width="100%" height="100%">
                            <AreaChart
                                data={dashboard.recentDays}
                                margin={{ top: 10, right: 12, left: 0, bottom: 0 }}
                            >
                                <defs>
                                    <linearGradient id="promptUsage" x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="5%" stopColor="#10b981" stopOpacity={0.24} />
                                        <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                                    </linearGradient>
                                    <linearGradient
                                        id="completionUsage"
                                        x1="0"
                                        y1="0"
                                        x2="0"
                                        y2="1"
                                    >
                                        <stop offset="5%" stopColor="#84cc16" stopOpacity={0.2} />
                                        <stop offset="95%" stopColor="#84cc16" stopOpacity={0} />
                                    </linearGradient>
                                </defs>
                                <CartesianGrid
                                    strokeDasharray="3 3"
                                    vertical={false}
                                    stroke="#d1fae5"
                                />
                                <XAxis
                                    dataKey="date"
                                    axisLine={false}
                                    tickLine={false}
                                    tick={{ fill: '#64748b', fontSize: 12 }}
                                    dy={10}
                                    tickFormatter={(date: string) => date.slice(5)}
                                />
                                <YAxis
                                    yAxisId="tokens"
                                    axisLine={false}
                                    tickLine={false}
                                    tick={{ fill: '#64748b', fontSize: 12 }}
                                    width={58}
                                    tickFormatter={(value: number) => formatCompactNumber(value)}
                                />
                                <YAxis yAxisId="requests" orientation="right" hide />
                                <Tooltip
                                    formatter={(value, name) => [formatNumber(Number(value)), name]}
                                    labelFormatter={(date) => t('dashboard.date', { date })}
                                    contentStyle={{
                                        borderRadius: '14px',
                                        border: '1px solid #a7f3d0',
                                        boxShadow: '0 12px 32px rgb(6 78 59 / 0.1)',
                                    }}
                                />
                                <Legend iconType="circle" wrapperStyle={{ paddingTop: '20px' }} />
                                <Area
                                    yAxisId="tokens"
                                    type="monotone"
                                    dataKey="promptTokens"
                                    name={t('dashboard.promptTokens')}
                                    stroke="#10b981"
                                    strokeWidth={2.5}
                                    fill="url(#promptUsage)"
                                />
                                <Area
                                    yAxisId="tokens"
                                    type="monotone"
                                    dataKey="completionTokens"
                                    name={t('dashboard.completionTokens')}
                                    stroke="#84cc16"
                                    strokeWidth={2.5}
                                    fill="url(#completionUsage)"
                                />
                                <Line
                                    yAxisId="requests"
                                    type="monotone"
                                    dataKey="requestCount"
                                    name={t('dashboard.requests')}
                                    stroke="#0d9488"
                                    strokeWidth={2}
                                    dot={{ r: 3, fill: '#0d9488' }}
                                />
                            </AreaChart>
                        </ResponsiveContainer>
                    </div>
                </section>

                <aside className="space-y-6">
                    <section className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm">
                        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">
                            {t('dashboard.todaysMix')}
                        </p>
                        <h2 className="mt-1 font-bold text-slate-900">
                            {t('dashboard.tokenComposition')}
                        </h2>
                        <div className="mt-6 flex items-center justify-center">
                            <div
                                className="relative flex h-36 w-36 items-center justify-center rounded-full"
                                style={{
                                    background: `conic-gradient(#f59e0b ${completionShare}%, #10b981 ${completionShare}% 100%)`,
                                }}
                            >
                                <div className="flex h-24 w-24 flex-col items-center justify-center rounded-full bg-white">
                                    <span className="text-2xl font-bold text-slate-900">
                                        {completionShare}%
                                    </span>
                                    <span className="text-[11px] text-slate-500">
                                        {t('dashboard.completion')}
                                    </span>
                                </div>
                            </div>
                        </div>
                        <div className="mt-6 grid grid-cols-2 gap-3 text-xs">
                            <div className="rounded-xl bg-emerald-50 p-3">
                                <span className="block h-2 w-2 rounded-full bg-emerald-500" />
                                <p className="mt-2 text-slate-500">{t('dashboard.prompt')}</p>
                                <p className="font-bold text-slate-800">{100 - completionShare}%</p>
                            </div>
                            <div className="rounded-xl bg-amber-50 p-3">
                                <span className="block h-2 w-2 rounded-full bg-amber-500" />
                                <p className="mt-2 text-slate-500">{t('dashboard.completion')}</p>
                                <p className="font-bold text-slate-800">{completionShare}%</p>
                            </div>
                        </div>
                    </section>
                    <section className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm">
                        <div className="flex items-center justify-between">
                            <div>
                                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-400">
                                    {t('dashboard.recentDays')}
                                </p>
                                <h2 className="mt-1 font-bold text-slate-900">
                                    {t('dashboard.dailyTotals')}
                                </h2>
                            </div>
                            {!hasUsage && (
                                <span className="rounded-full bg-slate-100 px-2 py-1 text-[10px] font-semibold text-slate-500">
                                    {t('dashboard.noUsage')}
                                </span>
                            )}
                        </div>
                        <div className="mt-4 space-y-3">
                            {dashboard.recentDays
                                .slice(-4)
                                .reverse()
                                .map((day) => (
                                    <div
                                        key={day.date}
                                        className="flex items-center justify-between border-b border-slate-100 pb-3 last:border-0 last:pb-0"
                                    >
                                        <div>
                                            <p className="text-sm font-semibold text-slate-700">
                                                {day.date.slice(5)}
                                            </p>
                                            <p className="text-xs text-slate-400">
                                                {t('dashboard.requestCount', {
                                                    count: formatNumber(day.requestCount),
                                                })}
                                            </p>
                                        </div>
                                        <p className="text-sm font-bold text-slate-900">
                                            {formatCompactNumber(day.totalTokens)}
                                        </p>
                                    </div>
                                ))}
                        </div>
                    </section>
                </aside>
            </div>
        </motion.div>
    );
}
