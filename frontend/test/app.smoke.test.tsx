// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import App from '../src/App'
import { ColorModeProvider } from '../src/store/mode'
import { I18nProvider } from '../src/i18n'
import { PreferencesProvider } from '../src/store/preferences'
import { SettingsProvider } from '../src/store/settings'
import { ToastProvider } from '../src/store/toast'

// A render pass over every page, against a stubbed API.
//
// The unit suites check the maths; this one checks that the components built on
// top of it mount at all. Several of the features added here — the widget
// frame, the command palette, the stage timeline — fail at render time or not
// at all, and a build that type-checks says nothing about that.

const OVERVIEW = {
  findings: 12, open_findings: 7, reachable_from_head: 3,
  repositories: 4, exposed_repos: 2, active_scans: 1, candidates: 5,
  notified_repos: 1, awaiting_notice: 1,
  by_severity: [{ key: 'critical', count: 2 }, { key: 'high', count: 3 }],
  by_provider: [{ key: 'github', count: 4 }, { key: 'unmapped', count: 3 }],
  by_status: [{ key: 'open', count: 7 }, { key: 'rotated', count: 5 }],
  by_scan_state: [{ key: 'completed', count: 2 }],
}

const FINDING = {
  id: 'f1', scan_job_id: 'j1',
  repository: { id: 'r1', owner: 'acme', name: 'web', url: 'https://github.com/acme/web' },
  rule_id: 'github-pat', provider: 'github', severity: 'critical', status: 'open',
  masked_preview: 'ghp_****abcd', file_path: 'config/app.yml', line_number: 12,
  commit_sha: 'abc1234', author_name: 'Dana', committed_at: '2026-01-02T03:04:05Z',
  reachable_from_head: true, detected_at: '2026-02-02T03:04:05Z',
  metadata: { probe_status: 'live', provider_name: 'GitHub', secret_length: '40', charset_class: 'alnum' },
}

const SCAN = {
  id: 'j1', state: 'completed', progress: 1, repos_total: 4, repos_scanned: 4,
  blobs_scanned: 9000, findings_count: 1,
  target: { kind: 'org', value: 'acme' },
  created_at: '2026-02-02T00:00:00Z', started_at: '2026-02-02T00:00:00Z', finished_at: '2026-02-02T00:30:00Z',
}

const REPOSITORY = {
  id: 'r1', owner: 'acme', name: 'web', url: 'https://github.com/acme/web',
  app_type: 'web', stars: 7, is_fork: false, last_scanned_at: '2026-02-02T00:30:00Z',
  findings: 3, open: 2, critical: 1, high: 1, medium: 0, low: 0,
}

/** Answers every endpoint the dashboard calls, by path. */
function stubFetch() {
  return vi.fn(async (url: RequestInfo | URL) => {
    const path = String(url).replace(/^https?:\/\/[^/]+/, '').split('?')[0]
    const body =
      path === '/v1/version'      ? { service: 'api', version: 'test', commit: 'abc', go_version: 'go1' }
      : path === '/v1/overview'   ? OVERVIEW
      : path === '/v1/analytics'  ? {
          timeline: [{ period: '2026-01', findings: 4 }], by_app_type: [{ key: 'web', count: 3 }],
          by_rule: [{ key: 'github-pat', count: 4 }], live_probes: [{ key: 'live', count: 1 }],
          notifications: [{ key: 'sent', count: 1 }],
        }
      : path === '/v1/scans'          ? { scans: [SCAN] }
      : path === '/v1/findings'       ? { findings: [FINDING], total: 1 }
      : path === '/v1/repositories'   ? { repositories: [REPOSITORY] }
      : path === '/v1/candidates'     ? { candidates: [] }
      : path === '/v1/providers'      ? { providers: [
          { slug: 'github', severity: 'high', rules: 1, rule_details: [{
            id: 'github-pat', severity: 'high', anchor: 'prefix', shape: 'gh[opsu]_[0-9A-Za-z]{36}',
            example: 'gh•_' + '•'.repeat(36), literals: ['ghp_', 'gho_'], keywords: [],
          }] },
          // An older api sends no rule_details; the dialog must still open.
          { slug: 'slack', severity: 'medium', rules: 2 },
        ] }
      : path === '/v1/notifications'  ? { notifications: [], total: 0, by_state: [] }
      : path === '/v1/settings'       ? { notifications_enabled: false, masking_enabled: true, unmask_owners: [], sealing_available: true }
      : {}
    return { ok: true, status: 200, json: async () => body } as Response
  })
}

const Harness = () => (
  <PreferencesProvider>
    <ColorModeProvider>
      <I18nProvider>
        <SettingsProvider>
          <ToastProvider>
            <App />
          </ToastProvider>
        </SettingsProvider>
      </I18nProvider>
    </ColorModeProvider>
  </PreferencesProvider>
)

/** Mounts at a route and lets the stubbed responses settle. */
async function open(hash: string) {
  window.location.hash = hash
  const view = render(<Harness />)
  await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve() })
  return view
}

beforeEach(() => {
  vi.stubGlobal('fetch', stubFetch())
  // jsdom implements no layout, so Element.scrollIntoView does not exist. The
  // palette calls it to keep the highlighted command in view.
  Element.prototype.scrollIntoView = () => {}
  // The tour owns the screen on a first visit, which would hide every page.
  localStorage.setItem('gs-prefs', JSON.stringify({ onboarded: true }))
  window.matchMedia = ((q: string) => ({
    matches: false, media: q, onchange: null,
    addEventListener: () => {}, removeEventListener: () => {},
    addListener: () => {}, removeListener: () => {}, dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
})

afterEach(() => { cleanup(); localStorage.clear(); vi.unstubAllGlobals() })

describe('every page mounts', () => {
  const pages: [string, string][] = [
    ['#/overview', 'Overview'],
    ['#/analytics', 'Analytics'],
    ['#/scans', 'Scans'],
    ['#/scans/j1', 'acme'],
    ['#/findings', 'Findings'],
    ['#/repositories', 'Repositories'],
    ['#/candidates', 'Unmapped'],
    ['#/notifications', 'Notifications'],
    ['#/providers', 'Providers'],
    ['#/settings', 'Settings'],
    ['#/help', 'Help'],
  ]

  for (const [hash, heading] of pages) {
    it(hash, async () => {
      await open(hash)
      expect(await screen.findAllByText(new RegExp(heading, 'i'))).not.toHaveLength(0)
    })
  }
})

describe('overview', () => {
  it('shows the posture score with its arithmetic behind a disclosure', async () => {
    await open('#/overview')
    expect(screen.getByText('/ 100')).toBeTruthy()
    const explain = screen.getByRole('button', { name: /how it is calculated/i })
    expect(explain.getAttribute('aria-expanded')).toBe('false')
    await act(async () => { fireEvent.click(explain) })
    expect(explain.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getAllByText(/Reachable from HEAD/i).length).toBeGreaterThan(0)
  })

  it('hides a widget and offers it back', async () => {
    await open('#/overview')
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /customise/i })) })
    const hide = screen.getAllByRole('button', { name: /^hide$/i })[0]
    await act(async () => { fireEvent.click(hide) })
    expect(screen.getByRole('button', { name: /\+ /i })).toBeTruthy()
    expect(JSON.parse(localStorage.getItem('gs-prefs')!).widgets).not.toContain('posture')
  })
})

describe('command palette', () => {
  it('opens on Ctrl+K and filters as you type', async () => {
    await open('#/overview')
    await act(async () => { fireEvent.keyDown(window, { key: 'k', ctrlKey: true }) })
    const box = screen.getByPlaceholderText(/search pages/i)
    await act(async () => { fireEvent.change(box, { target: { value: 'midnight' } }) })
    const list = screen.getByRole('listbox')
    expect(within(list).getAllByText(/Midnight/i).length).toBeGreaterThan(0)
  })

  it('applies a theme preset chosen from it', async () => {
    await open('#/overview')
    await act(async () => { fireEvent.keyDown(window, { key: 'k', ctrlKey: true }) })
    const box = screen.getByPlaceholderText(/search pages/i)
    await act(async () => { fireEvent.change(box, { target: { value: 'ocean' } }) })
    // Anchored: the free-text "search findings for …" row also contains the
    // query, and it sorts first.
    const option = within(screen.getByRole('listbox')).getByText(/^Theme: Ocean$/)
    await act(async () => { fireEvent.click(option) })
    expect(JSON.parse(localStorage.getItem('gs-prefs')!).themePreset).toBe('ocean')
  })

  it('does not steal a plain keystroke typed into a text field', async () => {
    await open('#/findings')
    const search = screen.getByLabelText(/search/i)
    await act(async () => { fireEvent.keyDown(search, { key: 'n' }) })
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})

describe('providers', () => {
  it('opens a provider and explains its keys, detection and severity', async () => {
    await open('#/providers')
    await act(async () => { fireEvent.click(await screen.findByRole('button', { name: /GitHub/ })) })
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('github-pat')).toBeTruthy()
    expect(within(dialog).getByText('gh•_' + '•'.repeat(36))).toBeTruthy()
    expect(within(dialog).getByText('own prefix')).toBeTruthy()
    expect(within(dialog).getByText(/Why high/i)).toBeTruthy()
    expect(within(dialog).getByText(/broad API access/i)).toBeTruthy()
    await act(async () => { fireEvent.click(within(dialog).getByRole('button', { name: /close/i })) })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('opens a provider the api sent without rule details', async () => {
    await open('#/providers')
    await act(async () => { fireEvent.click(await screen.findByRole('button', { name: /Slack/ })) })
    expect(within(screen.getByRole('dialog')).getByText(/does not report rule details/i)).toBeTruthy()
  })
})

describe('findings', () => {
  it('selects a row and offers bulk actions', async () => {
    await open('#/findings')
    const boxes = screen.getAllByRole('checkbox')
    await act(async () => { fireEvent.click(boxes[boxes.length - 1]) })
    expect(screen.getByText(/1 selected/i)).toBeTruthy()
  })

  it('opens the drawer and explains the match without the value', async () => {
    await open('#/findings?finding=f1')
    expect(await screen.findByText(/Why this was detected/i)).toBeTruthy()
    expect(screen.getAllByText(/ghp_\*\*\*\*abcd/).length).toBeGreaterThan(0)
    expect(screen.getByText(/What to do/i)).toBeTruthy()
  })
})

describe('settings', () => {
  it('switches the theme preset from the preview grid', async () => {
    await open('#/settings')
    const ocean = screen.getByRole('button', { name: /ocean/i })
    await act(async () => { fireEvent.click(ocean) })
    expect(JSON.parse(localStorage.getItem('gs-prefs')!).themePreset).toBe('ocean')
    expect(ocean.getAttribute('aria-pressed')).toBe('true')
  })

  it('resets every appearance preference at once', async () => {
    await open('#/settings')
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /compact/i })) })
    expect(JSON.parse(localStorage.getItem('gs-prefs')!).density).toBe('compact')
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /reset appearance/i })) })
    expect(JSON.parse(localStorage.getItem('gs-prefs')!).density).toBe('comfortable')
  })
})

describe('onboarding', () => {
  it('runs on a first visit and never again once finished', async () => {
    localStorage.clear()
    await open('#/overview')
    expect(screen.getByRole('dialog')).toBeTruthy()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /skip/i })) })
    // MUI keeps a closing dialog mounted for its exit transition, so the test
    // asks whether the tour's own heading is gone rather than the node.
    await waitFor(() => expect(screen.queryByText(/It clones a repository/i)).toBeNull())
    expect(JSON.parse(localStorage.getItem('gs-prefs')!).onboarded).toBe(true)
  })

  it('turns pages by button, arrow key, swipe and dot', async () => {
    localStorage.clear()
    await open('#/overview')
    const dialog = screen.getByRole('dialog')
    expect(screen.getByText('Step 1 of 7')).toBeTruthy()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /next/i })) })
    expect(screen.getByText('Step 2 of 7')).toBeTruthy()
    await act(async () => { fireEvent.keyDown(dialog, { key: 'ArrowLeft' }) })
    expect(screen.getByText('Step 1 of 7')).toBeTruthy()
    // A mostly vertical drag is a scroll, not a page turn.
    const art = screen.getByText(/It clones a repository/i)
    await act(async () => {
      fireEvent.touchStart(art, { touches: [{ clientX: 200, clientY: 100 }] })
      fireEvent.touchEnd(art, { changedTouches: [{ clientX: 140, clientY: 300 }] })
    })
    expect(screen.getByText('Step 1 of 7')).toBeTruthy()
    await act(async () => {
      fireEvent.touchStart(art, { touches: [{ clientX: 300, clientY: 100 }] })
      fireEvent.touchEnd(art, { changedTouches: [{ clientX: 100, clientY: 110 }] })
    })
    expect(screen.getByText('Step 2 of 7')).toBeTruthy()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /go to step 7/i })) })
    expect(screen.getByRole('button', { name: /get started/i })).toBeTruthy()
  })

  it('replays from settings, always from the first step', async () => {
    await open('#/settings')
    const replay = () => act(async () => { fireEvent.click(screen.getByRole('button', { name: /replay the introduction/i })) })
    await replay()
    expect(screen.getByText('Step 1 of 7')).toBeTruthy()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /next/i })) })
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /next/i })) })
    expect(screen.getByText('Step 3 of 7')).toBeTruthy()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: /skip/i })) })
    await waitFor(() => expect(screen.queryByText('Step 3 of 7')).toBeNull())
    await replay()
    expect(screen.getByText('Step 1 of 7')).toBeTruthy()
  })
})

describe('scan detail', () => {
  it('shows the stage timeline and the completion summary', async () => {
    await open('#/scans/j1')
    expect(screen.getAllByText(/Stages/i).length).toBeGreaterThan(0)
    expect(screen.getByText(/Discovering repositories/i)).toBeTruthy()
    expect(screen.getAllByText(/findings in .* repositories/i).length).toBeGreaterThan(0)
  })
})
