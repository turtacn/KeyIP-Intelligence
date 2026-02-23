interface ErrorLog {
  message: string;
  stack?: string;
  timestamp: string;
}

const errorBuffer: ErrorLog[] = [];
const MAX_LOGS = 50;

export const logError = (error: Error) => {
  const log: ErrorLog = {
    message: error.message,
    stack: error.stack,
    timestamp: new Date().toISOString(),
  };

  errorBuffer.unshift(log);
  if (errorBuffer.length > MAX_LOGS) {
    errorBuffer.pop();
  }

  console.error('[App Error]', log);
  // In production, send to tracking service here
};

export const getRecentErrors = () => [...errorBuffer];

// Global handlers
window.onerror = (message, _source, _lineno, _colno, error) => {
  logError(error || new Error(message as string));
};

window.onunhandledrejection = (event) => {
  logError(event.reason instanceof Error ? event.reason : new Error(String(event.reason)));
};
