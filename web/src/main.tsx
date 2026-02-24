import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'
import './i18n' // Import i18n config

// Prepare MSW
async function enableMocking() {
  const isMockMode = import.meta.env.VITE_API_MODE === 'mock';
  const isDev = import.meta.env.DEV;

  console.log('[App] Starting...', { isDev, isMockMode, VITE_API_MODE: import.meta.env.VITE_API_MODE });

  // If we are NOT in mock mode, and NOT in dev mode (unless dev mode is forcing mock), we don't start MSW.
  // But wait, the default behavior in Dockerfile is VITE_API_MODE=mock.

  if (!isMockMode && !isDev) {
    console.log('[App] MSW skipped: Production mode without VITE_API_MODE=mock');
    return;
  }

  // If explicitly 'real', skip
  if (import.meta.env.VITE_API_MODE === 'real') {
    console.log('[App] MSW skipped: VITE_API_MODE is real');
    return;
  }

  console.log('[App] Initializing MSW...');

  try {
    const { worker } = await import('./mocks/browser');

    // Start the worker
    await worker.start({
      onUnhandledRequest: 'bypass',
      serviceWorker: {
        url: '/mockServiceWorker.js',
      }
    });
    console.log('[App] MSW started successfully');
  } catch (error) {
    console.error('[App] Failed to start MSW:', error);
  }
}

enableMocking().then(() => {
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  )
});
