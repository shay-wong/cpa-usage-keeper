import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const devApiProxyTarget = process.env.VITE_API_PROXY_TARGET ?? 'http://127.0.0.1:8087'

export default defineConfig({
  base: './',
  plugins: [react()],
  server: {
    proxy: {
      '/api/v1': {
        target: devApiProxyTarget,
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
})
