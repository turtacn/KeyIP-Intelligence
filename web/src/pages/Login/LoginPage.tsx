import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { LogIn, AlertCircle } from 'lucide-react';
import { localSignIn, isLocalAuthAvailable } from '../../utils/auth';

const LoginPage: React.FC = () => {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  // Redirect if already logged in
  useEffect(() => {
    const token = sessionStorage.getItem('keyip_access_token');
    if (token) {
      navigate('/dashboard', { replace: true });
    }
  }, [navigate]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!email.trim() || !password) {
      setError('Email and password are required.');
      return;
    }

    setLoading(true);
    try {
      const result = await localSignIn(email.trim(), password);
      if (result.success) {
        navigate('/dashboard', { replace: true });
      } else {
        setError(result.error || 'Sign in failed. Please check your credentials.');
      }
    } catch {
      setError('Network error. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  if (!isLocalAuthAvailable()) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50">
        <div className="text-center max-w-md px-6">
          <LogIn className="w-12 h-12 text-slate-300 mx-auto mb-4" />
          <h1 className="text-xl font-semibold text-slate-700 mb-2">Sign In</h1>
          <p className="text-slate-500 text-sm">
            Local authentication is not available in Mock mode.
            Switch to Proxy mode in the API settings to sign in with your backend credentials.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-50">
      <div className="w-full max-w-sm mx-auto px-4">
        <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-6">
          <div className="text-center mb-6">
            <div className="w-12 h-12 bg-blue-100 rounded-full flex items-center justify-center mx-auto mb-3">
              <LogIn className="w-6 h-6 text-blue-600" />
            </div>
            <h1 className="text-xl font-bold text-slate-800">Sign In</h1>
            <p className="text-sm text-slate-500 mt-1">Enter your credentials to continue</p>
          </div>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg flex items-start gap-2">
              <AlertCircle className="w-4 h-4 text-red-500 shrink-0 mt-0.5" />
              <p className="text-sm text-red-700">{error}</p>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-slate-700 mb-1">
                Email
              </label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="admin@keyip.io"
                disabled={loading}
              />
            </div>

            <div>
              <label htmlFor="password" className="block text-sm font-medium text-slate-700 mb-1">
                Password
              </label>
              <input
                id="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="••••••••"
                disabled={loading}
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white text-sm font-medium rounded-lg transition-colors flex items-center justify-center gap-2"
            >
              {loading ? (
                <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
              ) : (
                <LogIn className="w-4 h-4" />
              )}
              {loading ? 'Signing in...' : 'Sign In'}
            </button>
          </form>

          <p className="text-xs text-slate-400 text-center mt-4">
            KeyIP Intelligence — Local Authentication
          </p>
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
