import type { NotificationType } from '../hooks/useNotification';

type NotifyFn = (
  type: NotificationType,
  title: string,
  message?: string,
  duration?: number,
) => void;

let notifyFn: NotifyFn | null = null;

/**
 * Register the global notification callback.
 * Called by NotificationProvider on mount.
 */
export function setNotifyFn(fn: NotifyFn | null): void {
  notifyFn = fn;
}

/**
 * Show a notification from non-React code (e.g., adapter.ts).
 * Safe to call even if the provider is not mounted (no-op).
 */
export function notify(
  type: NotificationType,
  title: string,
  message?: string,
  duration?: number,
): void {
  notifyFn?.(type, title, message, duration);
}
