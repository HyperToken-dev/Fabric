import type { ReactNode } from 'react';
import { AlertCircle, Inbox, RefreshCw } from 'lucide-react';

export function PageHeader({
    eyebrow,
    title,
    description,
    action,
}: {
    eyebrow: string;
    title: string;
    description: string;
    action?: ReactNode;
}) {
    return (
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
                    {eyebrow}
                </p>
                <h1 className="mt-1 text-3xl font-bold tracking-tight text-slate-900">{title}</h1>
                <p className="mt-1 text-slate-500">{description}</p>
            </div>
            {action}
        </div>
    );
}

export function ErrorState({ message, retry }: { message: string; retry: () => void }) {
    return (
        <div className="rounded-2xl border border-rose-100 bg-white p-8 text-center shadow-sm">
            <AlertCircle className="mx-auto text-rose-500" />
            <p className="mt-3 font-semibold text-slate-900">Unable to load data</p>
            <p className="mt-1 text-sm text-slate-500">{message}</p>
            <button
                type="button"
                onClick={retry}
                className="mt-4 inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2 text-sm font-semibold text-white"
            >
                <RefreshCw size={15} /> Retry
            </button>
        </div>
    );
}

export function EmptyState({ title, description }: { title: string; description: string }) {
    return (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-10 text-center">
            <Inbox className="mx-auto text-slate-400" />
            <p className="mt-3 font-semibold text-slate-800">{title}</p>
            <p className="mt-1 text-sm text-slate-500">{description}</p>
        </div>
    );
}

export function LoadingCards() {
    return (
        <div className="grid gap-4 animate-pulse md:grid-cols-2">
            {[0, 1, 2, 3].map((item) => (
                <div key={item} className="h-40 rounded-2xl border border-slate-100 bg-white" />
            ))}
        </div>
    );
}

export function RefreshWarning({ message, retry }: { message: string; retry: () => void }) {
    return (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            <span>{message}</span>
            <button type="button" onClick={retry} className="font-semibold underline">
                Retry
            </button>
        </div>
    );
}

export const inputClass =
    'w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm text-slate-900 outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100 disabled:bg-slate-100';
export const buttonClass =
    'inline-flex items-center justify-center rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50';
