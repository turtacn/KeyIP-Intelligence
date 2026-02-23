import React from 'react';
import { useLocation, Link } from 'react-router-dom';
import { ChevronRight, Home } from 'lucide-react';

const Breadcrumb: React.FC = () => {
  const location = useLocation();
  const pathnames = location.pathname.split('/').filter((x) => x);

  return (
    <nav className="flex items-center text-sm text-slate-500 mb-6" aria-label="Breadcrumb">
      <Link
        to="/"
        className="flex items-center hover:text-blue-600 transition-colors"
      >
        <Home className="w-4 h-4 mr-2" />
        Home
      </Link>
      {pathnames.map((name, index) => {
        const routeTo = `/${pathnames.slice(0, index + 1).join('/')}`;
        const isLast = index === pathnames.length - 1;

        return (
          <React.Fragment key={name}>
            <ChevronRight className="w-4 h-4 mx-2 text-slate-300" />
            {isLast ? (
              <span className="font-medium text-slate-800 capitalize">
                {name.replace(/-/g, ' ')}
              </span>
            ) : (
              <Link
                to={routeTo}
                className="hover:text-blue-600 transition-colors capitalize"
              >
                {name.replace(/-/g, ' ')}
              </Link>
            )}
          </React.Fragment>
        );
      })}
    </nav>
  );
};

export default Breadcrumb;
