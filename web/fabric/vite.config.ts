import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// https://vite.dev/config/
export default defineConfig({
    plugins: [react(), tailwindcss()],
    server: {
        proxy: {
            '/admin-api': {
                target: process.env.FABRIC_ADMIN_API_TARGET ?? 'http://localhost:9090',
                changeOrigin: true,
                rewrite: (path) => path.replace(/^\/admin-api/, ''),
            },
        },
    },
});
