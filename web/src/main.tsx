import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'
import './i18n' // Import i18n config

// Prepare MSW
async function enableMocking() {
  const isMockMode = import.meta.env.VITE_API_MODE === 'mock';
  const isDev = import.meta.env.DEV;

  // We enable mocking if:
  // 1. It's development mode (unless explicitly disabled, but we default to enabled for dev convenience if needed,
  //    though here we rely on VITE_API_MODE or just dev env).
  // 2. OR if VITE_API_MODE is explicitly 'mock' (this handles the Production Docker "Demo Mode").

  // If in prod and not mock mode, skip
  if (!isDev && !isMockMode) {
    return;
  }

  // If in dev, but VITE_API_MODE is set to something else (e.g. 'real'), skip
  if (isDev && import.meta.env.VITE_API_MODE && !isMockMode) {
    return;
  }

  const { worker } = await import('./mocks/browser')

  // Start the worker
  return worker.start({
    onUnhandledRequest: 'bypass',
    serviceWorker: {
      url: '/mockServiceWorker.js', // Explicit path ensures it works in nested routes if any, or standard location
    }
  })
}

enableMocking().then(() => {
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  )
})
