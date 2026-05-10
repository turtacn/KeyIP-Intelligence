/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_MODE: string;
  readonly VITE_API_BASE_URL: string;
  readonly VITE_API_LIVE_URL: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
