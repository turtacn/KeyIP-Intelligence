import React from 'react';
import { Link } from 'react-router-dom';
import { AlertCircle } from 'lucide-react';

const NotFound: React.FC = () => {
  return (
    <div className="min-h-[80vh] flex flex-col items-center justify-center text-center">
      <div className="bg-slate-100 p-6 rounded-full mb-6">
        <AlertCircle className="w-16 h-16 text-slate-400" />
      </div>
      <h1 className="text-4xl font-bold text-slate-900 mb-2">404</h1>
      <h2 className="text-xl font-semibold text-slate-700 mb-4">Page Not Found</h2>
      <p className="text-slate-500 max-w-md mb-8">
        The page you are looking for doesn't exist or has been moved.
      </p>
      <Link
        to="/"
        className="px-6 py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 transition-colors"
      >
        Return Home
      </Link>
    </div>
  );
};

export default NotFound;
