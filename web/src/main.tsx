import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'
import './i18n/i18n' // Import i18n config
import { registerServiceWorker } from './swRegister'

// Prepare MSW
async function enableMocking() {
  const { getApiMode } = await import('./utils/apiMode');
  const mode = getApiMode();

  console.log('[App] Starting...', { mode });

  // Only start MSW in mock mode
  if (mode !== 'mock') {
    console.log(`[App] MSW skipped (mode: ${mode})`);
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
  registerServiceWorker();
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  )
});
