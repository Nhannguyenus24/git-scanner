import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Most of the suite is DOM-free — preference migration, export masking, the
// posture maths, stage derivation, theme contrast and translation coverage are
// pure functions and data tables. app.smoke.test.tsx opts into jsdom with a
// file-level pragma to check that the components built on top of them render.
//
// The three build-time constants are defined here too. They come from
// vite.config.ts in a real build; without them the footer throws on first
// render and every mount test fails for the wrong reason.
export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify('test'),
    __APP_COMMIT__:  JSON.stringify('testing'),
    __BUILD_DATE__:  JSON.stringify('1970-01-01'),
  },
  test: {
    environment: 'node',
    include: ['test/**/*.test.ts', 'test/**/*.test.tsx'],
    restoreMocks: true,
  },
})
