/// <reference types="vitest" />
import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';

// В dev-режиме запросы /api и /health проксируются на Go-бэкенд,
// поэтому фронт и бэк работают с одного origin и CORS не мешает.
// Адрес бэкенда можно переопределить в frontend/.env.local: BACKEND_URL=http://localhost:8081
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const backend = env.BACKEND_URL || 'http://localhost:8080';
  return {
  plugins: [react()],
  server: {
    host: '127.0.0.1', // на Windows localhost может уйти в IPv6 (::1) и браузер не достучится
    port: 5173,
    proxy: {
      '/api': { target: backend, changeOrigin: true },
      '/health': { target: backend, changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'json-summary'],
      reportsDirectory: './coverage',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/main.tsx', 'src/test/**', 'src/**/*.test.{ts,tsx}', 'src/vite-env.d.ts'],
      thresholds: { lines: 80 },
    },
  },
  };
});
