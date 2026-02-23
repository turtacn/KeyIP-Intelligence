import React from 'react';
import { Bell, Search, User, Globe } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const TopBar: React.FC = () => {
  const { i18n, t } = useTranslation();

  const toggleLanguage = () => {
    const newLang = i18n.language === 'zh' ? 'en' : 'zh';
    i18n.changeLanguage(newLang);
  };

  return (
    <header className="h-16 bg-white border-b border-slate-200 sticky top-0 z-10 pl-64 transition-all duration-300">
      <div className="container mx-auto px-6 h-full flex items-center justify-between">
        <div className="flex items-center">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-5 h-5" />
            <input
              type="text"
              placeholder={t('dashboard.search_placeholder', 'Search patents, molecules, companies...')}
              className="pl-10 pr-4 py-2 border border-slate-300 rounded-lg w-96 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-shadow"
            />
          </div>
        </div>

        <div className="flex items-center space-x-4">
          <button
            onClick={toggleLanguage}
            className="flex items-center space-x-1 px-3 py-2 rounded-lg hover:bg-slate-100 text-slate-600 transition-colors"
            title="Switch Language"
          >
            <Globe className="w-5 h-5" />
            <span className="text-sm font-medium">{i18n.language === 'zh' ? 'EN' : '中文'}</span>
          </button>

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
