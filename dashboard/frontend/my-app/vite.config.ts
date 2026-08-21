import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import type { PluginOption } from 'vite'
import path from 'node:path'
import { mockFixturesPlugin } from './plugins/mock-fixtures.js'

export default defineConfig(({ mode }) => {
    const environment = loadEnv(mode, import.meta.dirname, '')
    const plugins: PluginOption[] = [react(), tailwindcss()]
    if (mode === 'mock') plugins.push(mockFixturesPlugin())
    return {
        plugins,
        resolve: {
            alias: {
                '@': path.resolve(import.meta.dirname, './src'),
            },
        },
        server: {
            hmr: mode === 'mock' ? false : undefined,
            proxy: {
                '/api': {
                    target: environment.VITE_API_PROXY_TARGET || 'http://localhost:8080',
                    changeOrigin: true,
                },
                '/grafana': {
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
