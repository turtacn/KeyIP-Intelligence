import React from 'react';
import { NavLink } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../../utils/auth';
import {
  LayoutDashboard,
  Search,
  ShieldAlert,
  Briefcase,
  Clock,
  Users,
  GitBranch,
  Globe,
  Activity,
} from 'lucide-react';

const Sidebar: React.FC = () => {
  const { t } = useTranslation();
  const { user, isAuthenticated } = useAuth();

  const navItems = [
    { to: '/dashboard', label: t('nav.dashboard'), icon: LayoutDashboard },
    { to: '/search', label: t('nav.search'), icon: Search },
    { to: '/knowledge-graph', label: t('nav.knowledge_graph'), icon: GitBranch },
    { to: '/patent-mining', label: t('nav.mining'), icon: Globe },
    { to: '/infringement-watch', label: t('nav.infringement'), icon: ShieldAlert },
    { to: '/portfolio', label: t('nav.portfolio'), icon: Briefcase },
    { to: '/lifecycle', label: t('nav.lifecycle'), icon: Clock },
    { to: '/partners', label: t('nav.partners'), icon: Users },
  ];

  return (
    <aside className="w-64 bg-slate-900 text-white flex flex-col h-screen fixed left-0 top-0 z-20 transition-all duration-300">
      <div className="h-16 flex items-center px-6 border-b border-slate-800">
        <span className="text-xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-blue-400 to-sky-400">
          {t('app.title')}
        </span>
      </div>

      <nav className="flex-1 py-6 px-3 space-y-1 overflow-y-auto">
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

      {/* Bottom navigation */}
      <div className="px-3 pb-2">
        <NavLink
          to="/health"
          className={({ isActive }) =>
            `flex items-center px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
              isActive
                ? 'bg-blue-800 text-white shadow-md'
                : 'text-slate-400 hover:bg-slate-800 hover:text-white'
            }`
          }
        >
          <Activity className="w-5 h-5 mr-3" />
          系统健康
        </NavLink>
      </div>

      <div className="p-4 border-t border-slate-800">
        <div className="flex items-center">
          <div className="w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-xs font-bold">
            {isAuthenticated && user
              ? (user.name || user.preferred_username || 'U')
                  .split(' ')
                  .map(s => s[0])
                  .join('')
                  .toUpperCase()
                  .slice(0, 2)
              : 'JD'}
          </div>
          <div className="ml-3">
            <p className="text-sm font-medium">
              {isAuthenticated && user
                ? user.name || user.preferred_username
                : 'John Doe'}
            </p>
            <p className="text-xs text-slate-500">
              {isAuthenticated && user?.email ? user.email : t('app.role')}
            </p>
          </div>
        </div>
      </div>
    </aside>
  );
};

export default Sidebar;
