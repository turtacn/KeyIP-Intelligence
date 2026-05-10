import React from 'react';
import { AlertTriangle, RotateCcw } from 'lucide-react';
import Button from './Button';

interface PageErrorProps {
  error: string;
  onRetry?: () => void;
  title?: string;
  description?: string;
}

const PageError: React.FC<PageErrorProps> = ({
  error,
  onRetry,
  title = 'Something went wrong',
  description,
}) => {
  return (
    <div className="min-h-[400px] flex flex-col items-center justify-center p-8 text-center">
      <div className="bg-red-50 p-4 rounded-full mb-4">
        <AlertTriangle className="w-10 h-10 text-red-500" />
      </div>
      <h2 className="text-xl font-bold text-slate-900 mb-2">{title}</h2>
      {description && (
        <p className="text-slate-500 mb-2 max-w-md">{description}</p>
      )}
      <p className="text-sm text-red-600 bg-red-50 px-4 py-2 rounded-lg mb-6 max-w-lg font-mono">
        {error}
      </p>
      {onRetry && (
        <Button
          onClick={onRetry}
          variant="outline"
          leftIcon={<RotateCcw className="w-4 h-4" />}
        >
          Retry
        </Button>
      )}
    </div>
  );
};

export default PageError;
