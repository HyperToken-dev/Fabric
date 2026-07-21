import { useState } from 'react';
import Sidebar from './components/Sidebar';
import Dashboard from './components/Dashboard';
import ChannelsPage from './components/ChannelsPage';
import ModelsPage from './components/ModelsPage';
import ApiKeysPage from './components/ApiKeysPage';
import UsageLogsPage from './components/UsageLogsPage';

const pages = {
    dashboard: Dashboard,
    channels: ChannelsPage,
    models: ModelsPage,
    'api-keys': ApiKeysPage,
    usage: UsageLogsPage,
};

export default function App() {
    const [activeTab, setActiveTab] = useState('dashboard');
    const ActivePage = pages[activeTab as keyof typeof pages] ?? Dashboard;

    return (
        <div className="min-h-screen bg-slate-50 font-sans md:flex">
            <Sidebar activeTab={activeTab} setActiveTab={setActiveTab} />

            <main className="min-w-0 flex-1 bg-slate-50 md:ml-64 md:h-screen md:overflow-y-auto">
                <ActivePage />
            </main>
        </div>
    );
}
