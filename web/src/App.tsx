import React from 'react';
import { RouterProvider } from 'react-router-dom';
import router from './router';
import { NotificationProvider } from './hooks/useNotification';
import { AuthProvider } from './utils/auth';
import { ToastContainer } from './components/ui/Toast';

const App: React.FC = () => {
  return (
    <AuthProvider>
      <NotificationProvider>
        <RouterProvider router={router} />
        <ToastContainer />
      </NotificationProvider>
    </AuthProvider>
  );
};

export default App;
