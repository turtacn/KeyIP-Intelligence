import { Component, ErrorInfo, ReactNode } from 'react';
import { AlertTriangle, RotateCcw } from 'lucide-react';
import Button from './Button';

interface Props {
  children?: ReactNode;
}

interface State {
  hasError: boolean;
  error?: Error;
}

class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("Uncaught error:", error, errorInfo);
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-[400px] flex flex-col items-center justify-center p-8 text-center bg-white rounded-lg border border-slate-200 shadow-sm">
          <div className="bg-red-50 p-4 rounded-full mb-4">
            <AlertTriangle className="w-10 h-10 text-red-500" />
          </div>
          <h2 className="text-xl font-bold text-slate-900 mb-2">Something went wrong</h2>
          <p className="text-slate-600 mb-6 max-w-md">
            {this.state.error?.message || "An unexpected error occurred while rendering this component."}
          </p>
          <Button
            onClick={() => window.location.reload()}
            variant="outline"
            leftIcon={<RotateCcw className="w-4 h-4" />}
          >
            Reload Page
          </Button>
        </div>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
