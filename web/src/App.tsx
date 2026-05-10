import React from 'react';
import { RouterProvider } from 'react-router-dom';
import router from './router';
import { NotificationProvider } from './hooks/useNotification';
import { ToastContainer } from './components/ui/Toast';

const App: React.FC = () => {
  return (
    <NotificationProvider>
      <RouterProvider router={router} />
      <ToastContainer />
    </NotificationProvider>
  );
};

export default App;
