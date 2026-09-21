import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  // Vite copies the versioned game assets into the embedded Wails bundle for
  // both development and production builds.
  publicDir: '../assets',
  plugins: [react(), tailwindcss()],
})
