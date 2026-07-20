import { useEffect, useState } from 'react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { Activity, Cpu, RefreshCw, Send, Zap } from 'lucide-react';
import { motion } from 'motion/react';
import { getUsageDashboard, type UsageDashboard } from '../api/dashboard';

const numberFormatter = new Intl.NumberFormat('en-US');

function formatNumber(value: number) {
  return numberFormatter.format(value);
}

export default function Dashboard() {
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
          setError(requestError instanceof Error ? requestError.message : 'Unable to load usage data');
        }
      });
    return () => controller.abort();
  }, [requestVersion]);

  if (!dashboard && !error) {
    return (
      <div className="p-8 max-w-7xl mx-auto space-y-8 animate-pulse" aria-label="Loading usage dashboard">
        <div className="h-9 w-72 rounded-lg bg-slate-200" />
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-5">
          {[0, 1, 2, 3].map((item) => <div key={item} className="h-36 rounded-2xl bg-white border border-slate-100" />)}
        </div>
        <div className="h-[420px] rounded-2xl bg-white border border-slate-100" />
      </div>
    );
  }

  if (!dashboard && error) {
    return (
      <div className="p-8 h-full flex items-center justify-center">
        <div className="max-w-md rounded-2xl border border-rose-100 bg-white p-8 text-center shadow-sm">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-rose-50 text-rose-600">
            <Activity size={24} />
          </div>
          <h1 className="text-xl font-bold text-slate-900">Usage data unavailable</h1>
          <p className="mt-2 text-sm text-slate-500">{error}</p>
          <button
            type="button"
            onClick={() => setRequestVersion((version) => version + 1)}
            className="mt-6 inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2.5 text-sm font-semibold text-white hover:bg-slate-800"
          >
            <RefreshCw size={16} /> Retry
          </button>
        </div>
      </div>
    );
  }

  if (!dashboard) {
    return null;
  }

  const stats = [
    { title: 'Tokens Today', value: dashboard.today.totalTokens, icon: Activity, color: 'text-indigo-600', bg: 'bg-indigo-50' },
    { title: 'Prompt Tokens', value: dashboard.today.promptTokens, icon: Cpu, color: 'text-emerald-600', bg: 'bg-emerald-50' },
    { title: 'Completion Tokens', value: dashboard.today.completionTokens, icon: Zap, color: 'text-amber-600', bg: 'bg-amber-50' },
    { title: 'Requests Today', value: dashboard.today.requestCount, icon: Send, color: 'text-sky-600', bg: 'bg-sky-50' },
  ];
  const hasUsage = dashboard.today.totalTokens > 0 || dashboard.recentDays.some((day) => day.totalTokens > 0);

  return (
    <motion.div
      initial={{ opacity: 0, y: 16 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35 }}
      className="p-5 md:p-8 max-w-7xl mx-auto space-y-7"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-indigo-600">Global usage</p>
          <h1 className="mt-1 text-3xl font-bold tracking-tight text-slate-900">Today's token pulse</h1>
          <p className="mt-1 text-slate-500">All gateway traffic, bucketed by the backend.</p>
        </div>
        <div className="rounded-xl border border-slate-200 bg-white px-4 py-2 text-sm text-slate-500 shadow-sm">
          Reporting time zone <span className="font-semibold text-slate-800">{dashboard.timeZone}</span>
        </div>
      </div>

      {error && (
        <div className="flex items-center justify-between rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          <span>Refresh failed: {error}. Showing the last loaded snapshot.</span>
          <button type="button" onClick={() => setRequestVersion((version) => version + 1)} className="font-semibold underline">Retry</button>
        </div>
      )}

      <div className="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-4">
        {stats.map((stat) => {
          const Icon = stat.icon;
          return (
            <div key={stat.title} className="rounded-2xl border border-slate-100 bg-white p-6 shadow-sm">
              <div className={`inline-flex rounded-xl p-3 ${stat.bg}`}><Icon className={`h-5 w-5 ${stat.color}`} /></div>
              <h2 className="mt-5 text-sm font-medium text-slate-500">{stat.title}</h2>
              <p className="mt-1 text-3xl font-bold tracking-tight text-slate-900">{formatNumber(stat.value)}</p>
            </div>
          );
        })}
      </div>

      <div className="rounded-2xl border border-slate-100 bg-white p-5 md:p-6 shadow-sm">
        <div className="mb-6 flex items-start justify-between gap-4">
          <div>
            <h2 className="text-lg font-bold text-slate-900">Last seven calendar days</h2>
            <p className="text-sm text-slate-500">Prompt and completion token volume</p>
          </div>
          {!hasUsage && <span className="rounded-full bg-slate-100 px-3 py-1 text-xs font-semibold text-slate-500">No usage recorded</span>}
        </div>
        <div className="h-[340px] w-full">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={dashboard.recentDays} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="promptUsage" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.28} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="completionUsage" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.28} />
                  <stop offset="95%" stopColor="#f59e0b" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0" />
              <XAxis dataKey="date" axisLine={false} tickLine={false} tick={{ fill: '#64748b', fontSize: 12 }} dy={10} tickFormatter={(date: string) => date.slice(5)} />
              <YAxis axisLine={false} tickLine={false} tick={{ fill: '#64748b', fontSize: 12 }} width={64} tickFormatter={formatNumber} />
              <Tooltip formatter={(value) => formatNumber(Number(value))} contentStyle={{ borderRadius: '12px', border: '1px solid #e2e8f0', boxShadow: '0 8px 24px rgb(15 23 42 / 0.08)' }} />
              <Legend iconType="circle" wrapperStyle={{ paddingTop: '20px' }} />
              <Area type="monotone" dataKey="promptTokens" name="Prompt Tokens" stroke="#10b981" strokeWidth={2} fill="url(#promptUsage)" />
              <Area type="monotone" dataKey="completionTokens" name="Completion Tokens" stroke="#f59e0b" strokeWidth={2} fill="url(#completionUsage)" />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>
    </motion.div>
  );
}
