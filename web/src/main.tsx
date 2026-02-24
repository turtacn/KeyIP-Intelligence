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

  // Logic:
  // 1. If VITE_API_MODE is 'mock', we MUST start MSW (Production Demo or Local Mock).
  // 2. If DEV mode and VITE_API_MODE is NOT 'real', we start MSW (Default Local Dev).

  const shouldStartMSW = isMockMode || (isDev && import.meta.env.VITE_API_MODE !== 'real');

  if (!shouldStartMSW) {
    console.log('[App] MSW skipped.');
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
