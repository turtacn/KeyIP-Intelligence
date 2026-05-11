import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import { visualizer } from 'rollup-plugin-visualizer'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    // Dev-server startup log: prints resolved API mode
    {
      name: 'api-mode-log',
      configureServer() {
        const mode = process.env.VITE_API_MODE || 'mock';
        console.log(`\n  🚀  API mode: ${mode}`);
        console.log(`  📡  Base URL: ${
          mode === 'mock'
            ? '/api/v1 (MSW)'
            : mode === 'proxy'
            ? 'http://localhost:8080/api/v1'
            : 'https://api.keyip.io/api/v1'
        }\n`);
      },
    },
    react(),
    ...(process.env.ANALYZE === 'true'
      ? [
          visualizer({
            filename: 'stats.html',
            template: 'treemap',
            open: false,
            gzipSize: true,
            brotliSize: true,
          }),
          visualizer({
            filename: 'stats.json',
            template: 'raw-data',
            open: false,
          }),
        ]
      : []),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          // Core React framework — loaded once, cached across pages
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
          // Internationalization
          i18n: ['i18next', 'i18next-browser-languagedetector', 'react-i18next'],
          // Heavy charting library (recharts + its d3 dependencies), shared by 3+ pages
          charts: ['recharts'],
          // Graph visualization, only used in KnowledgeGraph (lazy-loaded)
          cytoscape: ['cytoscape'],
          // RDKit WASM chemistry toolkit, only used in MoleculeViewer (lazy-loaded)
          rdkit: ['@rdkit/rdkit'],
        },
      },
    },
    // Increase warning threshold from default 500 KiB to 1 MiB
    // since we knowingly create dedicated chunks for heavy libs
    chunkSizeWarningLimit: 1000,
    // Minify CSS with esbuild (default: true)
    cssMinify: true,
    // No source maps in production
    sourcemap: false,
  },
})
