import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// In dev, `npm run dev` serves on :5173 and proxies API + streaming to nginx
// (localhost:80) so X-Accel-Redirect streaming works exactly like in prod.
export default defineConfig({
  plugins: [svelte()],
  server: {
    proxy: {
      '/api': 'http://localhost:80',
    },
  },
})
