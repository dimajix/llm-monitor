import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vuetify({ autoImport: true }),
  ],
  server: {
    port: 5173,
    proxy: {
      // Proxy API calls during dev to the Go API server
      '/api/v1': {
        target: process.env.VITE_API_BASE || 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
})