import React from 'react';
import { CheckCircle, XCircle, AlertTriangle, Info, X } from 'lucide-react';
import { useNotification, type NotificationType } from '../../hooks/useNotification';
import type { Notification } from '../../hooks/useNotification';

/* ---- Icon & style maps ---- */

const iconMap: Record<NotificationType, React.ReactNode> = {
  success: <CheckCircle className="w-5 h-5 text-green-500" aria-hidden="true" />,
  error: <XCircle className="w-5 h-5 text-red-500" aria-hidden="true" />,
  warning: <AlertTriangle className="w-5 h-5 text-amber-500" aria-hidden="true" />,
  info: <Info className="w-5 h-5 text-blue-500" aria-hidden="true" />,
};

const cardBgMap: Record<NotificationType, string> = {
  success: 'bg-green-50 dark:bg-green-950/40 border-green-200 dark:border-green-800',
  error: 'bg-red-50 dark:bg-red-950/40 border-red-200 dark:border-red-800',
  warning: 'bg-amber-50 dark:bg-amber-950/40 border-amber-200 dark:border-amber-800',
  info: 'bg-blue-50 dark:bg-blue-950/40 border-blue-200 dark:border-blue-800',
};

const progressColorMap: Record<NotificationType, string> = {
  success: 'bg-green-500',
  error: 'bg-red-500',
  warning: 'bg-amber-500',
  info: 'bg-blue-500',
};

/* ---- Single toast item ---- */

interface ToastItemProps {
  notification: Notification;
  onClose: (id: string) => void;
}

const ToastItem: React.FC<ToastItemProps> = ({ notification, onClose }) => {
  return (
    <div
      className={`
        flex items-start gap-3 p-4 rounded-lg border shadow-lg
        animate-slideInRight
        ${cardBgMap[notification.type]}
      `}
      role="alert"
    >
      {/* Icon */}
      <div className="flex-shrink-0 mt-0.5">{iconMap[notification.type]}</div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-gray-900 dark:text-gray-100">
          {notification.title}
        </p>
        {notification.message && (
          <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">
            {notification.message}
          </p>
        )}
      </div>

      {/* Close button */}
      <button
        onClick={() => onClose(notification.id)}
        className="flex-shrink-0 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors rounded p-0.5 hover:bg-black/5 dark:hover:bg-white/10"
        aria-label="Close notification"
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  );
};

/* ---- Progress bar (separate component for per-toast animation) ---- */

const ProgressBar: React.FC<{ notification: Notification }> = ({ notification }) => {
  return (
    <div className="absolute bottom-0 left-0 right-0 h-1 bg-black/10 dark:bg-white/10 overflow-hidden rounded-b-lg">
      <div
        key={notification.id}
        className={`h-full rounded-full ${progressColorMap[notification.type]}`}
        style={{
          animation: `shrink ${notification.duration}ms linear forwards`,
          transformOrigin: 'left center',
        }}
      />
    </div>
  );
};

/* ---- Container ---- */

export const ToastContainer: React.FC = () => {
  const { notifications, removeNotification } = useNotification();

  if (notifications.length === 0) return null;

  return (
    <div
      className="fixed top-4 right-4 z-[100] flex flex-col gap-2 max-w-sm w-full pointer-events-none"
      aria-live="polite"
      aria-label="Notifications"
    >
      {notifications.map((notification) => (
        <div key={notification.id} className="pointer-events-auto relative overflow-hidden rounded-lg">
          <ToastItem notification={notification} onClose={removeNotification} />
          {/* Only show progress bar for auto-dismissing notifications */}
          {notification.duration > 0 && <ProgressBar notification={notification} />}
        </div>
      ))}
    </div>
  );
};
