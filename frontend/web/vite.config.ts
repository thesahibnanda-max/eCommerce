import { fileURLToPath } from 'node:url'
import react from '@vitejs/plugin-react'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

const apiPort = process.env.AI_API_PORT ?? '3000'

function devApiPortMeta(): Plugin {
  return {
    name: 'ai-dev-api-port',
    apply: 'serve',
    transformIndexHtml() {
      return [{ tag: 'meta', attrs: { name: 'ai-api-port', content: apiPort }, injectTo: 'head' }]
    },
  }
}

function preloadFonts(): Plugin {
  return {
    name: 'ai-preload-fonts',
    apply: 'build',
    transformIndexHtml: {
      order: 'post',
      handler(_html, context) {
        const names = Object.keys(context.bundle ?? {}).filter((name) =>
          /(inter|fredoka)-latin-wght-normal-[\w-]+\.woff2$/.test(name),
        )
        return names.map((name) => ({
          tag: 'link',
          attrs: {
            rel: 'preload',
            as: 'font',
            type: 'font/woff2',
            crossorigin: '',
            href: `/${name}`,
          },
          injectTo: 'head' as const,
        }))
      },
    },
  }
}

export default defineConfig({
  plugins: [react(), devApiPortMeta(), preloadFonts()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { host: true, port: 4073, strictPort: true },
  preview: { host: true, port: 4073, strictPort: true },
  build: {
    outDir: '../staticFiles',
    emptyOutDir: true,
    sourcemap: false,
    assetsInlineLimit: 0,
    target: 'es2022',
    cssCodeSplit: true,
    modulePreload: { polyfill: false },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    css: { modules: { classNameStrategy: 'stable' } },
    restoreMocks: true,
  },
})
