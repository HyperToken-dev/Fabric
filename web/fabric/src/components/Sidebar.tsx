import {
    Activity,
    Boxes,
    FileJson,
    KeyRound,
    Languages,
    LayoutDashboard,
    LogOut,
    Radio,
    ShieldAlert,
} from 'lucide-react';
import type { NavigationItem } from '../types';
import { useI18n } from '../i18n';
import { isAdminUser, type CurrentUser } from '../api/auth';

interface SidebarProps {
    activeTab: string;
    setActiveTab: (tab: string) => void;
    user: CurrentUser;
    onLogout: () => void;
}

const navItems: (NavigationItem & { adminOnly?: boolean })[] = [
    { id: 'dashboard', label: 'nav.dashboard', icon: LayoutDashboard },
    { id: 'channels', label: 'nav.channels', icon: Radio, adminOnly: true },
    { id: 'models', label: 'nav.models', icon: Boxes },
    { id: 'api-keys', label: 'nav.apiKeys', icon: KeyRound },
    { id: 'usage', label: 'nav.usage', icon: Activity, adminOnly: true },
    { id: 'integral-logs', label: 'nav.integralLogs', icon: FileJson, adminOnly: true },
    { id: 'sensitive-words', label: 'nav.sensitiveWords', icon: ShieldAlert, adminOnly: true },
];

export default function Sidebar({ activeTab, setActiveTab, user, onLogout }: SidebarProps) {
    const { language, setLanguage, t } = useI18n();
    const isAdmin = isAdminUser(user);
    const nextLanguage = language === 'en-US' ? 'zh-CN' : 'en-US';
    const currentLanguageLabel =
        language === 'en-US' ? t('language.english') : t('language.chinese');
    const nextLanguageLabel =
        nextLanguage === 'en-US' ? t('language.english') : t('language.chinese');
    const languageButtonLabel = t('language.toggleAria', { language: nextLanguageLabel });
    const profileLabel = user.displayName || user.email || user.openid;
    const profileInitials =
        profileLabel
            .trim()
            .split(/\s+/)
            .map((part) => part[0])
            .join('')
            .slice(0, 2)
            .toUpperCase() || 'U';

    function toggleLanguage() {
        setLanguage(nextLanguage);
    }

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
                <div className="min-w-0">
                    <span className="block text-lg font-bold leading-tight tracking-tight text-emerald-950">
                        Fabric
                    </span>
                    <span className="block text-[10px] font-semibold uppercase tracking-[0.18em] text-emerald-600">
                        HyperToken
                    </span>
                </div>
            </div>

            <nav className="flex gap-2 overflow-x-auto px-4 py-3 md:flex-1 md:flex-col md:overflow-visible md:py-6 md:space-y-2">
                {navItems
                    .filter((item) => isAdmin || !item.adminOnly)
                    .map((item) => {
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
                <button
                    type="button"
                    onClick={toggleLanguage}
                    className="flex shrink-0 items-center gap-2 rounded-xl border border-emerald-100 bg-white/70 px-4 py-3 font-semibold text-emerald-700 transition hover:bg-emerald-100 hover:text-emerald-900 md:hidden"
                    aria-label={languageButtonLabel}
                    title={languageButtonLabel}
                >
                    <Languages size={20} />
                    <span className="text-xs">{currentLanguageLabel}</span>
                </button>
                <button
                    type="button"
                    onClick={onLogout}
                    className="flex shrink-0 items-center gap-2 rounded-xl border border-emerald-100 bg-white/70 px-4 py-3 font-semibold text-emerald-700 transition hover:bg-emerald-100 hover:text-emerald-900 md:hidden"
                    aria-label="Logout"
                    title="Logout"
                >
                    <LogOut size={20} />
                    <span className="text-xs">Logout</span>
                </button>
            </nav>

            <div className="hidden p-4 border-t border-emerald-100 md:block">
                <button
                    type="button"
                    onClick={toggleLanguage}
                    className="mb-3 flex w-full items-center justify-between rounded-xl border border-emerald-100 bg-white/70 px-4 py-3 text-sm font-semibold text-emerald-700 transition hover:bg-emerald-100 hover:text-emerald-900"
                    aria-label={languageButtonLabel}
                    title={languageButtonLabel}
                >
                    <span className="inline-flex items-center gap-2">
                        <Languages size={18} />
                        {t('language.label')}
                    </span>
                    <span className="text-xs font-bold">{currentLanguageLabel}</span>
                </button>
                <div className="flex items-center gap-3 px-4 py-3">
                    <div className="w-8 h-8 overflow-hidden rounded-full bg-emerald-600 flex items-center justify-center text-sm font-bold text-white shadow-sm shadow-emerald-200">
                        {user.avatarUrl ? (
                            <img
                                src={user.avatarUrl}
                                alt=""
                                className="h-full w-full object-cover"
                            />
                        ) : (
                            profileInitials
                        )}
                    </div>
                    <div className="flex flex-col text-sm text-left">
                        <span className="text-emerald-950 font-medium">{profileLabel}</span>
                        <span className="text-emerald-700/70 text-xs">
                            {user.email || user.openid}
                        </span>
                    </div>
                </div>
                <button
                    type="button"
                    onClick={onLogout}
                    className="mt-2 w-full rounded-xl border border-emerald-100 bg-white/70 px-4 py-2 text-sm font-semibold text-emerald-700 transition hover:bg-emerald-100 hover:text-emerald-900"
                >
                    Logout
                </button>
            </div>
        </aside>
    );
}
