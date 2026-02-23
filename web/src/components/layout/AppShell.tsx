import React, { Suspense } from 'react';
import Sidebar from './Sidebar';
import TopBar from './TopBar';
import Breadcrumb from './Breadcrumb';
import { Outlet } from 'react-router-dom';
import LoadingSpinner from '../ui/LoadingSpinner';

const AppShell: React.FC = () => {
  return (
    <div className="flex h-screen bg-slate-50 overflow-hidden font-sans">
      <Sidebar />

      <main className="flex-1 ml-64 flex flex-col transition-all duration-300">
        <TopBar />

        <div className="flex-1 overflow-y-auto p-8 relative">
          <div className="max-w-7xl mx-auto">
            <Breadcrumb />
            <Suspense fallback={
              <div className="flex items-center justify-center h-64">
                <LoadingSpinner size="lg" />
              </div>
            }>
              <Outlet />
            </Suspense>
          </div>
        </div>
      </main>
    </div>
  );
};

export default AppShell;
