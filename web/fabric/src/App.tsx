import { useState } from "react";
import Sidebar from "./components/Sidebar";
import Dashboard from "./components/Dashboard";

export default function App() {
  const [activeTab, setActiveTab] = useState('dashboard');

  return (
    <div className="min-h-screen bg-slate-50 flex font-sans">
      <Sidebar activeTab={activeTab} setActiveTab={setActiveTab} />

      <main className="flex-1 ml-64 overflow-y-auto h-screen bg-slate-50">
        {activeTab === 'dashboard' ? (
          <Dashboard />
        ) : (
          <div className="p-8 flex items-center justify-center h-full text-slate-400">
            <div className="text-center">
              <h2 className="text-2xl font-semibold mb-2 text-slate-600">Coming Soon</h2>
              <p>The {activeTab} view is currently under construction.</p>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
