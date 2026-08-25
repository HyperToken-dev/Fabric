import { useEffect, useState } from 'react';
import { getCurrentUser, isAdminUser, logout, type CurrentUser } from './api/auth';
import Sidebar from './components/Sidebar';
import Dashboard from './components/Dashboard';
import ChannelsPage from './components/ChannelsPage';
import ModelsPage from './components/ModelsPage';
import ClientModelsPage from './components/ClientModelsPage';
import ApiKeysPage from './components/ApiKeysPage';
import UsageLogsPage from './components/UsageLogsPage';
import IntegralLogsPage from './components/IntegralLogsPage';
import SensitiveWordsPage from './components/SensitiveWordsPage';

const pages = {
    dashboard: Dashboard,
    channels: ChannelsPage,
    models: ModelsPage,
    'integral-logs': IntegralLogsPage,
    'sensitive-words': SensitiveWordsPage,
};

export default function App() {
    const [activeTab, setActiveTab] = useState('dashboard');
    const [user, setUser] = useState<CurrentUser | null>(null);
    const [error, setError] = useState<string | null>(null);
    const isAdmin = isAdminUser(user);

    useEffect(() => {
        const controller = new AbortController();
        getCurrentUser(controller.signal)
            .then(setUser)
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted) {
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'Unable to load current user',
                    );
                }
            });
        return () => controller.abort();
    }, []);

    useEffect(() => {
        if (
            !isAdmin &&
            ['channels', 'usage', 'integral-logs', 'sensitive-words'].includes(activeTab)
        ) {
            setActiveTab('dashboard');
        }
    }, [activeTab, isAdmin]);

    if (!user) {
        return (
            <div className="flex min-h-screen items-center justify-center bg-slate-50 p-6">
                <div className="rounded-3xl border border-emerald-100 bg-white p-8 text-center shadow-sm">
                    <img
                        src="/Logo-HyperToken.png"
                        alt="HyperToken logo"
                        className="mx-auto h-14 w-14 object-contain"
                    />
                    <h1 className="mt-4 text-xl font-bold text-slate-900">Fabric</h1>
                    <p className="mt-2 text-sm text-slate-500">
                        {error ?? 'Loading your session...'}
                    </p>
                    {error && (
                        <a
                            href="/auth/login"
                            className="mt-5 inline-flex rounded-xl bg-emerald-600 px-4 py-2 text-sm font-semibold text-white"
                        >
                            Sign in
                        </a>
                    )}
                </div>
            </div>
        );
    }

    const ActivePage = pages[activeTab as keyof typeof pages] ?? Dashboard;
    const page =
        activeTab === 'api-keys' ? (
            <ApiKeysPage user={user} />
        ) : activeTab === 'usage' ? (
            <UsageLogsPage />
        ) : activeTab === 'models' && !isAdmin ? (
            <ClientModelsPage />
        ) : (
            <ActivePage />
        );

    return (
        <div className="min-h-screen bg-slate-50 font-sans md:flex">
            <Sidebar
                activeTab={activeTab}
                setActiveTab={setActiveTab}
                user={user}
                onLogout={() => void logout(user.oauthEnabled)}
            />

            <main className="min-w-0 flex-1 bg-slate-50 md:ml-64 md:h-screen md:overflow-y-auto">
                {page}
            </main>
        </div>
    );
}
