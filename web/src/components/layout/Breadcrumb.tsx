import React from 'react';
import { useLocation, Link } from 'react-router-dom';
import { ChevronRight, Home } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const Breadcrumb: React.FC = () => {
  const location = useLocation();
  const { t } = useTranslation();
  const pathnames = location.pathname.split('/').filter((x) => x);

  const routeNameMap: Record<string, string> = {
    'dashboard': 'nav.dashboard',
    'patent-mining': 'nav.mining',
    'infringement-watch': 'nav.infringement',
    'portfolio': 'nav.portfolio',
    'lifecycle': 'nav.lifecycle',
    'partners': 'nav.partners'
  };

  const getTranslatedName = (segment: string) => {
    const key = routeNameMap[segment];
    if (key) return t(key);
    // Fallback for unknown segments (e.g. IDs)
    return segment.charAt(0).toUpperCase() + segment.slice(1).replace(/-/g, ' ');
  };

  return (
    <nav className="flex items-center text-sm text-slate-500 mb-6" aria-label="Breadcrumb">
      <Link
        to="/"
        className="flex items-center hover:text-blue-600 transition-colors"
      >
        <Home className="w-4 h-4 mr-2" />
        {t('nav.home', 'Home')}
      </Link>
      {pathnames.map((name, index) => {
        const routeTo = `/${pathnames.slice(0, index + 1).join('/')}`;
        const isLast = index === pathnames.length - 1;

        return (
          <React.Fragment key={name}>
            <ChevronRight className="w-4 h-4 mx-2 text-slate-300" />
            {isLast ? (
              <span className="font-medium text-slate-800 capitalize">
                {getTranslatedName(name)}
              </span>
            ) : (
              <Link
                to={routeTo}
                className="hover:text-blue-600 transition-colors capitalize"
              >
                {getTranslatedName(name)}
              </Link>
            )}
          </React.Fragment>
        );
      })}
    </nav>
  );
};

export default Breadcrumb;
