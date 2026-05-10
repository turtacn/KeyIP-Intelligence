import React, { createContext, useContext, useState, useCallback, useRef, useEffect } from 'react';
import { setNotifyFn } from '../utils/notificationBridge';

export type NotificationType = 'success' | 'error' | 'warning' | 'info';

export interface Notification {
  id: string;
  type: NotificationType;
  title: string;
  message?: string;
  duration: number;
  createdAt: number;
}

interface NotificationContextValue {
  notifications: Notification[];
  addNotification: (
    type: NotificationType,
    title: string,
    message?: string,
    duration?: number,
  ) => void;
  removeNotification: (id: string) => void;
}

const NotificationContext = createContext<NotificationContextValue | undefined>(undefined);

export const NotificationProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const timersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const removeNotification = useCallback((id: string) => {
    setNotifications((prev) => prev.filter((n) => n.id !== id));
    const timer = timersRef.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timersRef.current.delete(id);
    }
  }, []);

  const addNotification = useCallback(
    (type: NotificationType, title: string, message?: string, duration?: number) => {
      const id =
        typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
          ? crypto.randomUUID()
          : Date.now().toString(36) + Math.random().toString(36).slice(2);
      const notification: Notification = {
        id,
        type,
        title,
        message,
        duration: duration ?? 5000,
        createdAt: Date.now(),
      };

      setNotifications((prev) => [...prev, notification]);

      const timer = setTimeout(() => {
        removeNotification(id);
      }, notification.duration);
      timersRef.current.set(id, timer);
    },
    [removeNotification],
  );

  // Wire up the global bridge for non-React code (e.g., adapter)
  useEffect(() => {
    setNotifyFn(addNotification);
    return () => {
      setNotifyFn(null);
    };
  }, [addNotification]);

  // Cleanup all timers on unmount
  useEffect(() => {
    return () => {
      timersRef.current.forEach((timer) => clearTimeout(timer));
      timersRef.current.clear();
    };
  }, []);

  return (
    <NotificationContext.Provider value={{ notifications, addNotification, removeNotification }}>
      {children}
    </NotificationContext.Provider>
  );
};

export function useNotification(): NotificationContextValue {
  const ctx = useContext(NotificationContext);
  if (!ctx) {
    throw new Error('useNotification must be used within a NotificationProvider');
  }
  return ctx;
}
