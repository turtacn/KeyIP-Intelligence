import React from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { AlertCircle } from 'lucide-react';

const NotFound: React.FC = () => {
  const { t } = useTranslation();
  return (
    <div className="min-h-[80vh] flex flex-col items-center justify-center text-center">
      <div className="bg-slate-100 p-6 rounded-full mb-6">
        <AlertCircle className="w-16 h-16 text-slate-400" />
      </div>
      <h1 className="text-4xl font-bold text-slate-900 mb-2">404</h1>
      <h2 className="text-xl font-semibold text-slate-700 mb-4">{t('not_found.title')}</h2>
      <p className="text-slate-500 max-w-md mb-8">
        {t('not_found.description')}
      </p>
      <Link
        to="/"
        className="px-6 py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 transition-colors"
      >
        {t('not_found.return_home')}
      </Link>
    </div>
  );
};

export default NotFound;
