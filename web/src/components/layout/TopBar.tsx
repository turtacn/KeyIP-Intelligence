import React, { useState, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Bell, Search, Globe, Settings, LogIn, LogOut } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { type ApiMode, getApiMode, setApiMode, isMockMode } from '../../utils/apiMode';
import { useAuth } from '../../utils/auth';
import { useAlerts } from '../../hooks/useAlerts';

const MODE_LABELS: Record<ApiMode, string> = {
  mock: 'Mock',
  proxy: 'Proxy',
  live: 'Live',
};

const MODE_TITLES: Record<ApiMode, string> = {
  mock: 'Mock (本地 Mock 数据)',
  proxy: 'Proxy (代理到 localhost:8080)',
  live: 'Live (直连生产环境)',
};

const ApiModeSwitcher: React.FC = () => {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const currentMode = getApiMode();

  // Close dropdown on outside click
  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [open]);

  const handleSelect = (mode: ApiMode) => {
    setOpen(false);
    setApiMode(mode); // triggers window.location.reload()
  };

  return (
    <div className="relative" ref={menuRef}>
      <button
        onClick={() => setOpen(!open)}
        className="p-2 text-slate-500 hover:bg-slate-100 rounded-full transition-colors relative"
        title={`API 模式: ${MODE_TITLES[currentMode]}`}
      >
        <Settings className="w-5 h-5" />
        {/* Small colored dot indicating current mode */}
        <span
          className={`absolute top-1.5 right-1.5 w-2 h-2 rounded-full border border-white ${
            currentMode === 'mock'
              ? 'bg-green-500'
              : currentMode === 'proxy'
              ? 'bg-amber-500'
              : 'bg-red-500'
          }`}
        />
      </button>

      {open && (
        <div className="absolute right-0 mt-2 w-52 bg-white rounded-lg shadow-lg border border-slate-200 py-1 z-50">
          <div className="px-4 py-2 text-xs font-semibold text-slate-400 uppercase tracking-wider border-b border-slate-100">
            API Mode
          </div>
          {(['mock', 'proxy', 'live'] as ApiMode[]).map((mode) => (
            <button
              key={mode}
              className={`w-full text-left px-4 py-2 text-sm flex items-center gap-3 transition-colors ${
                currentMode === mode
                  ? 'text-blue-600 bg-blue-50 font-medium'
                  : 'text-slate-700 hover:bg-slate-50'
              }`}
              onClick={() => handleSelect(mode)}
            >
              <span
                className={`w-2.5 h-2.5 rounded-full shrink-0 ${
                  mode === 'mock'
                    ? 'bg-green-500'
                    : mode === 'proxy'
                    ? 'bg-amber-500'
                    : 'bg-red-500'
                }`}
              />
              <span className="flex-1">{MODE_LABELS[mode]}</span>
              <span className="text-xs text-slate-400 font-normal">
                {mode === 'mock' ? '本地' : mode === 'proxy' ? '代理' : '生产'}
              </span>
            </button>
          ))}
          <div className="px-4 py-2 text-xs text-slate-400 border-t border-slate-100 mt-1">
            {MODE_TITLES[currentMode]}
          </div>
        </div>
      )}
    </div>
  );
};

const AuthButton: React.FC = () => {
  const { isAuthenticated: authed, user, login, logout } = useAuth();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Do not show auth UI in mock mode
  if (isMockMode()) {
    return null;
  }

  // Close menu on outside click
  useEffect(() => {
    if (!menuOpen) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [menuOpen]);

  if (!authed || !user) {
    return (
      <button
        onClick={() => login()}
        className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-blue-600 bg-blue-50 hover:bg-blue-100 rounded-lg transition-colors"
        title="Sign in with Keycloak"
      >
        <LogIn className="w-4 h-4" />
        <span>Sign In</span>
      </button>
    );
  }

  // Get initials for avatar fallback
  const initials = (user.name || user.preferred_username || 'U')
    .split(' ')
    .map(s => s[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);

  return (
    <div className="relative" ref={menuRef}>
      <button
        onClick={() => setMenuOpen(!menuOpen)}
        className="flex items-center gap-2 p-1 pr-3 rounded-full hover:bg-slate-50 transition-colors"
      >
        <div className="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-sm font-medium">
          {initials}
        </div>
        <span className="text-sm text-slate-700 max-w-[120px] truncate hidden sm:inline">
          {user.name || user.preferred_username}
        </span>
      </button>

      {menuOpen && (
        <div className="absolute right-0 mt-2 w-56 bg-white rounded-lg shadow-lg border border-slate-200 py-1 z-50">
          <div className="px-4 py-3 border-b border-slate-100">
            <p className="text-sm font-medium text-slate-800 truncate">{user.name || user.preferred_username}</p>
            <p className="text-xs text-slate-500 truncate">{user.email}</p>
          </div>
          <button
            onClick={() => logout()}
            className="w-full flex items-center gap-3 px-4 py-2.5 text-sm text-slate-700 hover:bg-slate-50 transition-colors"
          >
            <LogOut className="w-4 h-4" />
            <span>Sign Out</span>
          </button>
        </div>
      )}
    </div>
  );
};

const AlertBell: React.FC = () => {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const { alerts, unreadCount, loading } = useAlerts();

  // Close dropdown on outside click
  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [open]);

  // Only show in non-mock mode
  if (isMockMode()) return null;

  const recentAlerts = alerts.slice(0, 5);

  const severityDot: Record<string, string> = {
    high: 'bg-red-500',
    medium: 'bg-amber-500',
    low: 'bg-blue-500',
  };

  return (
    <div className="relative" ref={menuRef}>
      <button
        onClick={() => setOpen(!open)}
        className="p-2 text-slate-500 hover:bg-slate-100 rounded-full transition-colors relative"
        title="Notifications"
      >
        <Bell className="w-5 h-5" />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 min-w-[18px] h-[18px] flex items-center justify-center bg-red-500 text-white text-[10px] font-bold rounded-full px-1 border-2 border-white">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 mt-2 w-80 bg-white rounded-lg shadow-lg border border-slate-200 py-1 z-50">
          <div className="px-4 py-2 text-xs font-semibold text-slate-400 uppercase tracking-wider border-b border-slate-100">
            Notifications
          </div>

          {loading ? (
            <div className="px-4 py-8 text-center text-sm text-slate-400">
              <div className="animate-pulse">Loading...</div>
            </div>
          ) : recentAlerts.length === 0 ? (
            <div className="px-4 py-8 text-center text-sm text-slate-400">
              No new notifications
            </div>
          ) : (
            recentAlerts.map((alert) => (
              <div
                key={alert.id}
                className={`px-4 py-3 border-b border-slate-50 last:border-b-0 hover:bg-slate-50 transition-colors cursor-pointer ${
                  !alert.read ? 'bg-blue-50/40' : ''
                }`}
              >
                <div className="flex items-start gap-3">
                  <span
                    className={`mt-1.5 w-2 h-2 rounded-full shrink-0 ${
                      severityDot[alert.severity] ?? 'bg-slate-400'
                    }`}
                  />
                  <div className="flex-1 min-w-0">
                    <p
                      className={`text-sm ${
                        !alert.read
                          ? 'font-semibold text-slate-800'
                          : 'text-slate-600'
                      }`}
                    >
                      {alert.title}
                    </p>
                    {alert.message && (
                      <p className="text-xs text-slate-400 mt-0.5 line-clamp-2">
                        {alert.message}
                      </p>
                    )}
                    <p className="text-xs text-slate-400 mt-1">
                      {new Date(alert.createdAt).toLocaleDateString()}
                    </p>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
};

const TopBar: React.FC = () => {
  const { i18n, t } = useTranslation();
  const navigate = useNavigate();
  const [searchValue, setSearchValue] = useState('');

  const changeLanguage = (e: React.ChangeEvent<HTMLSelectElement>) => {
    i18n.changeLanguage(e.target.value);
  };

  const handleSearchKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      const trimmed = searchValue.trim();
      if (trimmed) {
        navigate(`/search?q=${encodeURIComponent(trimmed)}`);
      } else {
        navigate('/search');
      }
    }
  };

  return (
    <header className="h-16 bg-white border-b border-slate-200 sticky top-0 z-10 transition-all duration-300">
      <div className="container mx-auto px-6 h-full flex items-center justify-between">
        <div className="flex items-center">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-5 h-5" />
            <input
              type="text"
              value={searchValue}
              onChange={(e) => setSearchValue(e.target.value)}
              onKeyDown={handleSearchKeyDown}
              placeholder={t('dashboard.search_placeholder', 'Search patents, molecules, companies...')}
              className="pl-10 pr-4 py-2 border border-slate-300 rounded-lg w-96 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-shadow"
            />
          </div>
        </div>

        <div className="flex items-center space-x-4">
          {/* API Mode Switcher */}
          <ApiModeSwitcher />

          <div className="relative flex items-center">
            <Globe className="w-5 h-5 text-slate-600 absolute left-3 pointer-events-none" />
            <select
              value={i18n.language}
              onChange={changeLanguage}
              className="pl-10 pr-4 py-2 bg-transparent rounded-lg hover:bg-slate-100 text-slate-600 text-sm font-medium appearance-none focus:outline-none focus:ring-2 focus:ring-blue-500 cursor-pointer"
              title="Switch Language"
            >
              <option value="zh-CN">中文 (简体)</option>
              <option value="en">English</option>
              <option value="ja">日本語</option>
              <option value="ko">한국어</option>
            </select>
          </div>

          <AlertBell />

          <div className="h-8 w-px bg-slate-200 mx-2"></div>

          {/* Auth: login/logout button or user avatar */}
          <AuthButton />
        </div>
      </div>
    </header>
  );
};

export default TopBar;
