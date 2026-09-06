import { defineConfig } from 'vite';

export default defineConfig({
  build: { target: 'es2020' },
  server: { proxy: { '/api': 'http://127.0.0.1:8080' } },
});
