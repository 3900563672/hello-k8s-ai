import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

export default defineConfig(({ mode }) => {
    const environment = loadEnv(mode, import.meta.dirname, '')
    return {
        plugins: [react(), tailwindcss()],
        resolve: {
            alias: {
                '@': path.resolve(import.meta.dirname, './src'),
            },
        },
        server: {
            proxy: {
                '/api': {
                    target: environment.VITE_API_PROXY_TARGET || 'http://localhost:8080',
                    changeOrigin: true,
                },
            },
        },
        build: {
            chunkSizeWarningLimit: 900,
        },
    }
})
