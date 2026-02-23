import React from 'react';
import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Search,
  ShieldAlert,
  Briefcase,
  Clock,
  Users
} from 'lucide-react';

const Sidebar: React.FC = () => {
  const navItems = [
    { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { to: '/patent-mining', label: 'Patent Mining', icon: Search },
    { to: '/infringement-watch', label: 'Infringement Watch', icon: ShieldAlert },
    { to: '/portfolio', label: 'Portfolio Optimizer', icon: Briefcase },
    { to: '/lifecycle', label: 'Lifecycle Mgmt', icon: Clock },
    { to: '/partners', label: 'Partner Portal', icon: Users },
  ];

  return (
    <aside className="w-64 bg-slate-900 text-white flex flex-col h-screen fixed left-0 top-0 z-20 transition-all duration-300">
      <div className="h-16 flex items-center px-6 border-b border-slate-800">
        <span className="text-xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-blue-400 to-sky-400">
          KeyIP Intelligence
        </span>
      </div>

      <nav className="flex-1 py-6 px-3 space-y-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              `flex items-center px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-blue-800 text-white shadow-md'
                  : 'text-slate-400 hover:bg-slate-800 hover:text-white'
              }`
            }
          >
            <item.icon className="w-5 h-5 mr-3" />
            {item.label}
          </NavLink>
        ))}
      </nav>

      <div className="p-4 border-t border-slate-800">
        <div className="flex items-center">
          <div className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-xs font-bold">
            JD
          </div>
          <div className="ml-3">
            <p className="text-sm font-medium">John Doe</p>
            <p className="text-xs text-slate-500">IP Manager</p>
          </div>
        </div>
      </div>
    </aside>
  );
};

export default Sidebar;
