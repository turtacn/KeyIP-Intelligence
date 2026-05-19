import React from 'react';
import { createBrowserRouter, Navigate } from 'react-router-dom';
import AppShell from './components/layout/AppShell';
import ErrorBoundary from './components/ui/ErrorBoundary';

// Lazy load pages
const LoginEntry = React.lazy(() => import('./pages/Login/LoginEntry'));
const ExecutiveDashboard = React.lazy(() => import('./pages/ExecutiveDashboard'));
const PatentMining = React.lazy(() => import('./pages/PatentMining'));
const InfringementWatch = React.lazy(() => import('./pages/InfringementWatch'));
const PortfolioOptimizer = React.lazy(() => import('./pages/PortfolioOptimizer'));
const LifecycleConsole = React.lazy(() => import('./pages/LifecycleConsole'));
const PartnerPortal = React.lazy(() => import('./pages/PartnerPortal'));
const Search = React.lazy(() => import('./pages/Search'));
const KnowledgeGraph = React.lazy(() => import('./pages/KnowledgeGraph'));
const Health = React.lazy(() => import('./pages/Health'));
const PatentDetail = React.lazy(() => import('./pages/PatentDetail'));
const MoleculeDetail = React.lazy(() => import('./pages/MoleculeDetail'));
const Molecules = React.lazy(() => import('./pages/Molecules'));
const FTOSearch = React.lazy(() => import('./pages/FTOSearch'));
const Workspaces = React.lazy(() => import('./pages/Workspaces'));
const Reports = React.lazy(() => import('./pages/Reports'));
const Settings = React.lazy(() => import('./pages/Settings'));
const NotFound = React.lazy(() => import('./pages/NotFound'));

const router = createBrowserRouter([
  {
    path: '/login',
    element: (
      <ErrorBoundary>
        <LoginEntry />
      </ErrorBoundary>
    ),
  },
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
        path: 'search',
        element: <Search />,
      },
      {
        path: 'knowledge-graph',
        element: <KnowledgeGraph />,
      },
      {
        path: 'patents/:id',
        element: <PatentDetail />,
      },
      {
        path: 'molecules',
        element: <Molecules />,
      },
      {
        path: 'molecules/:id',
        element: <MoleculeDetail />,
      },
      {
        path: 'fto',
        element: <FTOSearch />,
      },
      {
        path: 'workspaces',
        element: <Workspaces />,
      },
      {
        path: 'reports',
        element: <Reports />,
      },
      {
        path: 'settings',
        element: <Settings />,
      },
      {
        path: 'health',
        element: <Health />,
      },
      {
        path: '*',
        element: <NotFound />,
      },
    ],
  },
]);

export default router;
