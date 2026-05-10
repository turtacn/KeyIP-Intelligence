import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { handleCallback, getLoginRedirect } from '../../utils/auth';
import LoadingSpinner from '../../components/ui/LoadingSpinner';

const LoginCallback: React.FC = () => {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);
  const [retrying, setRetrying] = useState(false);

  useEffect(() => {
    let cancelled = false;

    const processCallback = async () => {
      setRetrying(true);

      const result = await handleCallback();

      if (cancelled) return;

      if (result.success) {
        // Redirect to the original page, or dashboard
        const redirectTo = getLoginRedirect() || '/dashboard';
        navigate(redirectTo, { replace: true });
      } else {
        setError(result.error || 'Authentication failed. Please try again.');
        setRetrying(false);
      }
    };

    processCallback();

    return () => {
      cancelled = true;
    };
  }, [navigate]);

  const handleRetry = () => {
    setError(null);
    window.location.href = '/login';
  };

  const handleBackToHome = () => {
    navigate('/', { replace: true });
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-50">
      <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-8 max-w-md w-full mx-4">
        {error ? (
          <div className="text-center">
            <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-red-100 flex items-center justify-center">
              <svg className="w-8 h-8 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </div>
            <h2 className="text-xl font-semibold text-slate-800 mb-2">Authentication Failed</h2>
            <p className="text-sm text-slate-600 mb-6 break-all">{error}</p>
            <div className="flex gap-3 justify-center">
              <button
                onClick={handleRetry}
                disabled={retrying}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 transition-colors text-sm font-medium"
              >
                {retrying ? 'Retrying...' : 'Try Again'}
              </button>
              <button
                onClick={handleBackToHome}
                className="px-4 py-2 border border-slate-300 text-slate-700 rounded-lg hover:bg-slate-50 transition-colors text-sm font-medium"
              >
                Back to Home
              </button>
            </div>
          </div>
        ) : (
          <div className="text-center">
            <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-blue-100 flex items-center justify-center">
              <LoadingSpinner size="md" />
            </div>
            <h2 className="text-xl font-semibold text-slate-800 mb-2">Signing In</h2>
            <p className="text-sm text-slate-500">
              Completing authentication, please wait...
            </p>
          </div>
        )}
      </div>
    </div>
  );
};

export default LoginCallback;
