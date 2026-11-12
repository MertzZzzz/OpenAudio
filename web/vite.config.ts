/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// BASE_DOMAIN (e.g. openaudio.example.com) lets the dev server answer on that host —
// Vite rejects unknown Host headers otherwise. Extra hosts: VITE_ALLOWED_HOSTS=a.example,b.example.
const baseHost = (process.env.BASE_DOMAIN ?? '').trim().toLowerCase().replace(/:\d+$/, '');
const allowedHosts = [baseHost, ...(process.env.VITE_ALLOWED_HOSTS ?? '').split(',')]
  .map((h) => h.trim())
  .filter(Boolean);

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    host: true,
    port: 5173,
    allowedHosts,
    // The Go API is served under the web origin so auth cookies are first-party
    // and OAuth redirect URIs look like http://localhost:5173/api/v1/auth/oauth/<provider>/callback.
    proxy: {
      '/api': { target: process.env.VITE_API_PROXY ?? 'http://localhost:8080', changeOrigin: false, xfwd: true },
    },
  },
  preview: { allowedHosts },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
});
