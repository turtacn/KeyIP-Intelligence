import React, { Suspense } from 'react';
import LoadingSpinner from '../../components/ui/LoadingSpinner';

const LoginCallback = React.lazy(() => import('./LoginCallback'));
const LoginPage = React.lazy(() => import('./LoginPage'));

/**
 * LoginEntry: decides whether to render the OIDC callback or the local login form.
 *
 * - If the URL contains `code` or `error` query params, it's an OAuth callback → LoginCallback
 * - Otherwise, render the local email+password LoginPage
 */
const LoginEntry: React.FC = () => {
  const params = new URLSearchParams(window.location.search);
  const isOAuthCallback = params.has('code') || params.has('error');

  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center bg-slate-50">
          <LoadingSpinner size="md" />
        </div>
      }
    >
      {isOAuthCallback ? <LoginCallback /> : <LoginPage />}
    </Suspense>
  );
};

export default LoginEntry;
