import {
    Activity,
    Boxes,
    FileJson,
    KeyRound,
    LayoutDashboard,
    Radio,
    ShieldAlert,
} from 'lucide-react';
import type { NavigationItem } from '../types';
import { useI18n, type Language } from '../i18n';

interface SidebarProps {
    activeTab: string;
    setActiveTab: (tab: string) => void;
}

const navItems: NavigationItem[] = [
    { id: 'dashboard', label: 'nav.dashboard', icon: LayoutDashboard },
    { id: 'channels', label: 'nav.channels', icon: Radio },
    { id: 'models', label: 'nav.models', icon: Boxes },
    { id: 'api-keys', label: 'nav.apiKeys', icon: KeyRound },
    { id: 'usage', label: 'nav.usage', icon: Activity },
    { id: 'integral-logs', label: 'nav.integralLogs', icon: FileJson },
    { id: 'sensitive-words', label: 'nav.sensitiveWords', icon: ShieldAlert },
];

const languages: { language: Language; labelKey: string }[] = [
    { language: 'en-US', labelKey: 'language.english' },
    { language: 'zh-CN', labelKey: 'language.chinese' },
];

export default function Sidebar({ activeTab, setActiveTab }: SidebarProps) {
    const { language, setLanguage, t } = useI18n();

    return (
        <aside className="w-full bg-gradient-to-b from-emerald-50 via-teal-50 to-white text-slate-600 flex flex-col border-r border-emerald-100 md:h-screen md:w-64 md:fixed md:top-0 md:left-0">
            <div className="h-20 px-5 flex items-center gap-3 border-b border-emerald-100">
                <div className="h-11 w-11 shrink-0 overflow-hidden rounded-xl bg-white p-1.5 shadow-lg shadow-emerald-200/60 ring-1 ring-emerald-100">
                    <img
                        src="/Logo-HyperToken.png"
                        alt="HyperToken logo"
                        className="h-full w-full object-contain"
                    />
                </div>
                <div className="min-w-0 flex-1">
                    <span className="block text-lg font-bold leading-tight tracking-tight text-emerald-950">
                        Fabric
                    </span>
                    <span className="block text-[10px] font-semibold uppercase tracking-[0.18em] text-emerald-600">
                        HyperToken
                    </span>
                </div>
                <div className="flex shrink-0 overflow-hidden rounded-full border border-emerald-200 bg-white/70 p-0.5 text-[11px] font-bold text-emerald-700 shadow-sm">
                    {languages.map((item) => (
                        <button
                            key={item.language}
                            type="button"
                            onClick={() => setLanguage(item.language)}
                            className={`rounded-full px-2 py-1 transition ${
                                language === item.language
                                    ? 'bg-emerald-600 text-white shadow-sm'
                                    : 'hover:bg-emerald-50'
                            }`}
                        >
                            {t(item.labelKey)}
                        </button>
                    ))}
                </div>
            </div>

            <nav className="flex gap-2 overflow-x-auto px-4 py-3 md:flex-1 md:flex-col md:overflow-visible md:py-6 md:space-y-2">
                {navItems.map((item) => {
                    const Icon = item.icon;
                    const isActive = activeTab === item.id;
                    return (
                        <button
                            key={item.id}
                            onClick={() => setActiveTab(item.id)}
                            className={`shrink-0 flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 md:w-full ${
                                isActive
                                    ? 'bg-emerald-600 text-white shadow-lg shadow-emerald-200/80'
                                    : 'hover:bg-emerald-100 hover:text-emerald-900'
                            }`}
                        >
                            <Icon size={20} />
                            <span className="font-medium">{t(item.label)}</span>
                        </button>
                    );
                })}
            </nav>

            <div className="hidden p-4 border-t border-emerald-100 md:block">
                <div className="flex items-center gap-3 px-4 py-3">
                    <div className="w-8 h-8 rounded-full bg-emerald-600 flex items-center justify-center text-sm font-bold text-white shadow-sm shadow-emerald-200">
                        AD
                    </div>
                    <div className="flex flex-col text-sm text-left">
                        <span className="text-emerald-950 font-medium">
                            {t('sidebar.adminUser')}
                        </span>
                        <span className="text-emerald-700/70 text-xs">admin@example.com</span>
                    </div>
                </div>
            </div>
        </aside>
    );
}
