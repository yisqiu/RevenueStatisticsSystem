import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from '@vben/vite-config';

export default defineConfig(async () => ({
  application: { archiver: false, devtools: false, extraAppConfig: false, injectAppLoading: false, injectMetadata: false, nitroMock: false, pwa: false },
  vite: {
    resolve: { alias: { '#': fileURLToPath(new URL('./src', import.meta.url)) } },
    server: { host: '127.0.0.1', port: 5327, strictPort: true, proxy: { '/api': { target: 'http://127.0.0.1:8127', changeOrigin: false } } },
  },
}));
