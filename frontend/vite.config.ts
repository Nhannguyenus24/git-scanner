import { execSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The app's version is stamped at build time so the running bundle can say which
// build it is — the same reason the Go services carry buildinfo. It comes from
// package.json, with the short git sha and build date appended, and each is a
// best-effort read: a build outside a git checkout (a release tarball, CI
// without history) still succeeds, just without the commit.
const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8')) as { version: string }

function git(cmd: string): string {
  try {
    return execSync(`git ${cmd}`, { stdio: ['ignore', 'pipe', 'ignore'] }).toString().trim()
  } catch {
    return ''
  }
}

const commit = process.env.VITE_COMMIT ?? git('rev-parse --short HEAD')
const buildDate = new Date().toISOString().slice(0, 10)

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
    __APP_COMMIT__:  JSON.stringify(commit),
    __BUILD_DATE__:  JSON.stringify(buildDate),
  },
  server: {
    port: 3000,
    proxy: {
      '/v1':      { target: 'http://localhost:8080', changeOrigin: true },
      '/healthz': { target: 'http://localhost:8080', changeOrigin: true },
      '/readyz':  { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
