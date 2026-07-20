import { LayoutDashboard, Settings, Users, FileText } from 'lucide-react';
import type { NavigationItem } from '../types';

interface SidebarProps {
  activeTab: string;
  setActiveTab: (tab: string) => void;
}

const navItems: NavigationItem[] = [
  { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { id: 'users', label: 'Users', icon: Users },
  { id: 'documents', label: 'Documents', icon: FileText },
  { id: 'settings', label: 'Settings', icon: Settings },
];

export default function Sidebar({ activeTab, setActiveTab }: SidebarProps) {
  return (
    <div className="w-64 bg-slate-900 text-slate-300 flex flex-col h-screen fixed top-0 left-0 border-r border-slate-800">
      <div className="h-20 px-5 flex items-center gap-3 border-b border-slate-800">
        <div className="h-11 w-11 shrink-0 overflow-hidden rounded-xl bg-white p-1.5 shadow-lg shadow-black/20 ring-1 ring-white/10">
          <img
            src="/Logo-HyperToken.png"
            alt="HyperToken logo"
            className="h-full w-full object-contain"
          />
        </div>
        <div className="min-w-0">
          <span className="block text-lg font-bold leading-tight tracking-tight text-white">Fabric</span>
          <span className="block text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-500">HyperToken</span>
        </div>
      </div>
      
      <nav className="flex-1 px-4 py-6 space-y-2">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeTab === item.id;
          return (
            <button
              key={item.id}
              onClick={() => setActiveTab(item.id)}
              className={`w-full flex items-center gap-3 px-4 py-3 rounded-xl transition-all duration-200 ${
                isActive
                  ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-900/20'
                  : 'hover:bg-slate-800 hover:text-white'
              }`}
            >
              <Icon size={20} />
              <span className="font-medium">{item.label}</span>
            </button>
          );
        })}
      </nav>
      
      <div className="p-4 border-t border-slate-800">
        <div className="flex items-center gap-3 px-4 py-3">
          <div className="w-8 h-8 rounded-full bg-slate-700 flex items-center justify-center text-sm font-bold text-white">
            AD
          </div>
          <div className="flex flex-col text-sm text-left">
            <span className="text-white font-medium">Admin User</span>
            <span className="text-slate-500 text-xs">admin@example.com</span>
          </div>
        </div>
      </div>
    </div>
  );
}
