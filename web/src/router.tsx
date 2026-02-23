import React from 'react';
import { createBrowserRouter, Navigate } from 'react-router-dom';
import AppShell from './components/layout/AppShell';
import ErrorBoundary from './components/ui/ErrorBoundary';

// Lazy load pages
const ExecutiveDashboard = React.lazy(() => import('./pages/ExecutiveDashboard'));
const PatentMining = React.lazy(() => import('./pages/PatentMining'));
const InfringementWatch = React.lazy(() => import('./pages/InfringementWatch'));
const PortfolioOptimizer = React.lazy(() => import('./pages/PortfolioOptimizer'));
const LifecycleConsole = React.lazy(() => import('./pages/LifecycleConsole'));
const PartnerPortal = React.lazy(() => import('./pages/PartnerPortal'));
const NotFound = React.lazy(() => import('./pages/NotFound'));

// Placeholders for detail pages (Phase 3)
const PatentDetail = React.lazy(() => import('./pages/NotFound')); // Stub
const MoleculeDetail = React.lazy(() => import('./pages/NotFound')); // Stub

const router = createBrowserRouter([
  {
    path: '/',
    element: (
      <ErrorBoundary>
        <AppShell />
      </ErrorBoundary>
    ),
    children: [
      {
        index: true,
        element: <Navigate to="/dashboard" replace />,
      },
      {
        path: 'dashboard',
        element: <ExecutiveDashboard />,
      },
      {
        path: 'patent-mining',
        element: <PatentMining />,
      },
      {
        path: 'infringement-watch',
        element: <InfringementWatch />,
      },
      {
        path: 'portfolio',
        element: <PortfolioOptimizer />,
      },
      {
        path: 'lifecycle',
        element: <LifecycleConsole />,
      },
      {
        path: 'partners',
        element: <PartnerPortal />,
      },
      {
        path: 'patents/:id',
        element: <PatentDetail />,
      },
      {
        path: 'molecules/:id',
        element: <MoleculeDetail />,
      },
      {
        path: '*',
        element: <NotFound />,
      },
    ],
  },
]);

export default router;
