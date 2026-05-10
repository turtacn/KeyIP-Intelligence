import React from 'react';
import { AlertTriangle, RotateCcw } from 'lucide-react';
import Button from './Button';

interface ErrorFallbackProps {
  error: Error | null;
  resetErrorBoundary?: () => void;
}

const ErrorFallback: React.FC<ErrorFallbackProps> = ({ error, resetErrorBoundary }) => {
  return (
    <div className="min-h-[400px] flex flex-col items-center justify-center p-8 text-center bg-white rounded-lg border border-slate-200 shadow-sm">
      <div className="bg-red-50 p-4 rounded-full mb-4">
        <AlertTriangle className="w-10 h-10 text-red-500" />
      </div>
      <h2 className="text-xl font-bold text-slate-900 mb-2">Something went wrong</h2>
      <p className="text-slate-600 mb-6 max-w-md">
        {error?.message || 'An unexpected error occurred while rendering this component.'}
      </p>
      {resetErrorBoundary && (
        <Button
          onClick={resetErrorBoundary}
          variant="outline"
          leftIcon={<RotateCcw className="w-4 h-4" />}
        >
          Try Again
        </Button>
      )}
      {!resetErrorBoundary && (
        <Button
          onClick={() => window.location.reload()}
          variant="outline"
          leftIcon={<RotateCcw className="w-4 h-4" />}
        >
          Reload Page
        </Button>
      )}
    </div>
  );
};

export default ErrorFallback;
