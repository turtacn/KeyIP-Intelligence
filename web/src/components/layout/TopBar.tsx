import React, { useState, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Bell, Search, User, Globe, Settings } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { type ApiMode, getApiMode, setApiMode } from '../../utils/apiMode';

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

          <button className="p-2 text-slate-500 hover:bg-slate-100 rounded-full transition-colors relative">
            <Bell className="w-5 h-5" />
            <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-red-500 rounded-full border border-white"></span>
          </button>

          <div className="h-8 w-px bg-slate-200 mx-2"></div>

          <button className="flex items-center space-x-2 p-1 pr-3 rounded-full hover:bg-slate-50 transition-colors">
            <div className="w-8 h-8 rounded-full bg-slate-200 flex items-center justify-center text-slate-600">
              <User className="w-5 h-5" />
            </div>
          </button>
        </div>
      </div>
    </header>
  );
};

export default TopBar;
