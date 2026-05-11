import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import { visualizer } from 'rollup-plugin-visualizer'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
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
