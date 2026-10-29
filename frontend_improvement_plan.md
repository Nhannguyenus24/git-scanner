# GitScanner Frontend Improvement Plan

This plan turns the proposed frontend enhancements into an incremental roadmap
for the React/MUI application in this directory.

The current frontend already provides:

- Overview, analytics, scans, findings, repositories, candidates, notifications,
  providers, settings, and help pages.
- Light/dark mode with a custom paper-and-ink visual style.
- Seven UI languages.
- API connection status and browser-persisted API URL.
- Findings masking and disclosure-owner settings.
- Scan progress and analytics visualizations.

The plan prioritizes improvements that make the dashboard more distinctive and
useful without weakening the security model or requiring backend changes before
they are needed.

---

## 1. Product goals

### Primary goals

1. Make the dashboard feel configurable and personal.
2. Reduce the time needed to investigate and triage a finding.
3. Make long-running scans easier to understand.
4. Turn raw security data into clear risk and remediation priorities.
5. Preserve the existing visual identity, responsive behavior, localization, and
   secret-masking rules.

### Non-goals

- Do not display or persist plaintext credentials.
- Do not add client-side job or finding data as a second source of truth.
- Do not add live credential verification from the browser.
- Do not introduce a second design system or replace MUI.
- Do not make animation necessary for understanding scan state.

---

## 2. Delivery principles

- Ship in small vertical slices that can be reviewed and tested independently.
- Prefer local preferences in `localStorage` for presentation-only settings.
- Keep server-backed state in the API and refresh it through the existing hooks.
- Add translations for every user-visible string.
- Respect reduced-motion and high-contrast accessibility preferences.
- Keep the current paper theme as the default.
- Use progressive enhancement: tables and filters must remain usable without
  advanced drag/drop, animation, or chart interaction.
- Add frontend-only features first where possible, then extend API contracts only
  when the feature needs server data.

---

## 3. Suggested delivery phases

| Phase | Theme | Main outcome |
|---|---|---|
| 0 | Foundation | Shared preference, layout, shortcut, and accessibility primitives |
| 1 | Personalization | Theme presets, density, sidebar, dashboard layout |
| 2 | Investigation workflow | Saved filters, bulk actions, finding investigation improvements |
| 3 | Scan visibility | Scan timeline, activity feed, richer scan detail |
| 4 | Risk intelligence | Posture score, repository risk profiles, interactive analytics |
| 5 | Operations | Notification preferences, exports, reporting |
| 6 | Product polish | Onboarding, responsive refinements, accessibility hardening |

Phases 1 and 2 provide the highest immediate user value. Phase 0 should be
completed first because later phases depend on its preference and accessibility
patterns.

---

## 4. Phase 0 — Frontend foundation

### 4.1 Add a preference model

Create a typed preference model for presentation-only settings:

```ts
type ThemePreset = 'paper' | 'midnight' | 'ocean' | 'signal' | 'minimal' | 'high-contrast'
type Density = 'comfortable' | 'compact'

interface UiPreferences {
  themePreset: ThemePreset
  colorMode: 'light' | 'dark' | 'system'
  density: Density
  reducedMotion: boolean | 'system'
  sidebarCollapsed: boolean
}
```

Store this separately from API settings. API URL and server settings should
remain distinct from browser-only preferences.

### 4.2 Build shared UI primitives

Add reusable components/hooks for:

- `PreferenceRow`
- `ThemePreview`
- `DashboardWidget`
- `ShortcutHint`
- `EmptyState`
- `FilterBar`
- `SavedFilterMenu`
- `ResponsiveDrawer`

### 4.3 Accessibility baseline

- Add visible focus styles.
- Ensure all icon-only buttons have translated accessible labels.
- Add keyboard navigation to menus, filters, tables, and drawers.
- Respect `prefers-reduced-motion`.
- Add a high-contrast palette with tested text and chart contrast.
- Ensure charts have text summaries or accessible labels.

### Acceptance criteria

- Existing pages continue to render with no preference selected.
- Preferences survive reload and invalid stored values fall back safely.
- Keyboard-only users can navigate the top bar, sidebar, dialogs, tables, and
  finding drawer.
- `npm run build` succeeds.

---

## 5. Phase 1 — Personalization

## 5.1 Theme presets

Extend [src/theme.ts](./src/theme.ts) so the theme is generated from both mode
and preset rather than mode alone.

Recommended presets:

| Preset | Character |
|---|---|
| Paper | Current vintage technical-instrument style |
| Midnight | Near-black terminal with green/cyan accents |
| Ocean | Navy surfaces with cyan and blue accents |
| Signal | Dark security-operations style with orange/red alerts |
| Minimal | Clean, restrained enterprise dashboard |
| High contrast | WCAG-oriented palette with stronger borders and text |

Requirements:

- Keep severity semantics consistent across presets.
- Do not use color as the only severity indicator.
- Keep chart colors readable on both light and dark surfaces.
- Show a live preview in settings.
- Persist the selected preset locally.

## 5.2 Expanded appearance settings

Add an Appearance section to [src/pages/SettingsPage.tsx](./src/pages/SettingsPage.tsx):

- Theme preset selector
- Light/dark/system mode
- Comfortable/compact density
- Sidebar collapsed/expanded
- Reduced motion
- High-contrast mode, either as a preset or accessibility toggle

The existing dark-mode toggle in
[src/store/mode.tsx](./src/store/mode.tsx) should remain compatible during the
migration. Existing `gs-mode` values should be migrated without surprising the
user.

## 5.3 Customizable dashboard

Allow users to:

- Show or hide overview widgets.
- Reorder widgets.
- Choose from layouts such as:
  - Daily triage
  - Executive summary
  - Security operations
- Reset to the default layout.

Start with buttons or menu-based ordering before introducing drag-and-drop. A
drag-and-drop interaction may be added after the widget model is stable.

### Acceptance criteria

- The current dashboard remains the default.
- A user can change appearance settings without a page reload.
- A user can reset all appearance and dashboard preferences.
- Mobile layouts remain usable and do not require horizontal scrolling for core
  content.
- Theme previews never expose secret values or server data.

---

## 6. Phase 2 — Investigation workflow

## 6.1 Saved finding filters

Extend the findings page with:

- Multi-filter support for severity, status, provider, repository, rule, and
  reachability.
- Presets:
  - Critical open findings
  - Reachable credentials
  - Unmapped candidates
  - Recently discovered
  - Old unresolved findings
- Save, rename, delete, and reset filters.
- URL synchronization so filtered views can be bookmarked and shared.

Saved filters should initially be browser-local. Add server-backed saved filters
only if users need them across browsers or teams.

## 6.2 Bulk triage

Add selection and bulk actions:

- Mark as reviewed.
- Mark as false positive.
- Mark as rotated.
- Assign an owner, if supported by the API.
- Export selected findings.

Actions must show confirmation for destructive or irreversible changes and
refresh the server-backed list after completion.

## 6.3 Finding investigation drawer

Improve [src/components/FindingDrawer.tsx](./src/components/FindingDrawer.tsx) with:

- Finding lifecycle timeline.
- First seen and last seen.
- Repository and branch context.
- Reachability and risk explanation.
- Provider classification and rule details.
- Duplicate or related finding references.
- Clear remediation steps.
- Copy-safe actions that never copy or reveal a secret by default.

Add a “Why was this detected?” section that explains the rule, confidence, and
relevant metadata without exposing the credential itself.

## 6.4 Command palette

Add a global `Ctrl+K` / `Cmd+K` command palette:

- Navigate to any page.
- Start a new scan.
- Search findings.
- Open saved filters.
- Switch theme or color mode.
- Switch language.
- Toggle sidebar and density.
- Export the current view.
- Open help and settings.

Commands should be permission-aware and should not bypass server authorization.

### Acceptance criteria

- Filter state is reproducible from the URL.
- Bulk actions cannot silently fail; errors are shown through the existing toast
  system.
- Finding details remain masked according to server settings.
- Command palette is keyboard accessible and does not interfere with text input.
- Existing finding links continue to open the same filtered views.

---

## 7. Phase 3 — Scan visibility and activity

## 7.1 Rich scan detail timeline

Extend the scan detail page with explicit stages:

1. Discovering repositories
2. Cloning
3. Walking history
4. Detecting credentials
5. Classifying providers
6. Finalizing analytics
7. Completed or failed

Display:

- Current stage.
- Repository progress.
- Findings discovered over time.
- Blobs scanned.
- Estimated remaining time when reliable.
- Paused/resumed state.
- Failed repositories with retry actions.
- Live event stream.

Do not invent an ETA when the server does not provide enough information; show
stage and counters instead.

## 7.2 Security activity feed

Add a compact overview activity feed for:

- New critical finding.
- Scan completion.
- Scan failure.
- Repository added.
- Provider classification.
- Notification sent.
- Candidate mapping.
- Scan pause/resume.

Use server events or existing timestamps. Avoid fake client-generated events.

## 7.3 Scan completion feedback

Add subtle, optional completion feedback:

- Status transition animation.
- Summary of findings.
- Direct links to critical findings and affected repositories.
- Reduced-motion fallback with no animation.

### Acceptance criteria

- Scan progress is understandable without animation.
- Refreshing the page does not lose scan state.
- Failed stages expose an actionable error.
- Activity items link to the relevant scan, finding, repository, or notification.
- No secret material appears in progress logs or activity labels.

---

## 8. Phase 4 — Risk intelligence

## 8.1 Security posture score

Add a score to the overview page:

```text
Security posture: 72 / 100
```

Break the score into explainable factors:

- Open findings.
- Critical and high severity findings.
- Reachable credentials.
- Age of unresolved findings.
- Exposed repositories.
- Unmapped candidates.
- Scan coverage.
- Notification status.

Show trend information only when the API provides a meaningful comparison:

```text
72 / 100  ↑ 8 this month
```

The score must be explainable, not a black box. The calculation should live in
the backend when it depends on authoritative data; the frontend should render
the score and explanation.

## 8.2 Repository risk profiles

Improve the repositories page and repository detail view with:

- Risk score.
- Open and critical findings.
- Reachable credentials.
- Last scan and scan coverage.
- Owner notification status.
- Most affected branches.
- Finding history.
- Oldest unresolved finding.
- Recommended next action.

## 8.3 Interactive analytics

Extend [src/pages/AnalyticsPage.tsx](./src/pages/AnalyticsPage.tsx) with:

- Date range selector.
- Compare-two-periods mode.
- Repository, provider, severity, and status filters.
- Click-to-filter chart interactions.
- Exact-value tooltips.
- Top repositories by exposure.
- Oldest unresolved findings.
- Most frequently detected providers.
- Export chart data as CSV.

Use a shared filter state so analytics links to findings with equivalent filters.

### Acceptance criteria

- Every score has a visible explanation.
- Date ranges and filters are reflected in the URL or clearly resettable.
- Charts remain usable on mobile and with reduced motion.
- Exported values match the currently displayed filters.
- The frontend does not calculate authoritative security metrics from partial
  client data when the API can provide them.

---

## 9. Phase 5 — Operations and reporting

## 9.1 Notification preferences

Expand settings beyond the current notification switch:

- Critical/high-only notifications.
- Reachable-credential notifications.
- Scan failure notifications.
- Scan completion notifications.
- Daily or weekly digest.
- Channel preferences.
- Per-repository overrides.

Keep masking and disclosure-owner controls separate from notification delivery
preferences.

## 9.2 Export and reporting

Add export actions for:

- CSV findings.
- JSON scan data.
- Markdown incident report.
- PDF security report, if a supported report service exists.
- Executive summary.
- Provider exposure report.
- Repository remediation report.

Reports should contain masked values or fingerprints only. Exports must clearly
show the active filters and generation timestamp.

## 9.3 Report history

If report generation becomes asynchronous, add a report history view with:

- Requested time.
- Report type.
- Filters used.
- Status.
- Download action.
- Expiration or retention information.

### Acceptance criteria

- Exports never contain plaintext credentials.
- Export failures are explicit and retryable.
- Notification preferences are reflected by the server after save.
- Reports identify their scope, filters, and generation time.

---

## 10. Phase 6 — Onboarding and responsive polish

## 10.1 First-run onboarding

Create a short guided flow:

1. Configure the API URL.
2. Check the API connection.
3. Start the first scan.
4. Explain overview cards.
5. Demonstrate finding triage.
6. Explain masking and disclosure.
7. Recommend notification setup.

Allow users to skip, restart, or disable the tour. Store only completion state
locally.

## 10.2 Responsive improvements

Improve the current desktop sidebar/mobile tab behavior:

- Collapsible desktop sidebar.
- Bottom navigation for small screens.
- Mobile filter drawer.
- Sticky table headers.
- Card-based finding rows on narrow screens.
- Responsive chart simplification.
- Swipe or full-screen finding drawer on mobile.

## 10.3 Visual polish

- Consistent loading skeletons.
- Empty states with useful next actions.
- Clear error recovery actions.
- Subtle status transitions.
- Consistent hover, focus, selected, and disabled states.
- Optional density-aware spacing.

### Acceptance criteria

- Core workflows work at mobile widths without clipped controls.
- The onboarding flow can be completed using keyboard navigation.
- Reduced motion removes non-essential transitions.
- Empty and error states always provide a useful next action where possible.

---

## 11. Proposed implementation surfaces

| Capability | Likely files |
|---|---|
| Theme presets | `src/theme.ts`, `src/store/mode.tsx`, `src/store/preferences.tsx` |
| Appearance settings | `src/pages/SettingsPage.tsx`, `src/i18n.tsx` |
| Dashboard layout | `src/pages/OverviewPage.tsx`, `src/components/DashboardWidget.tsx` |
| Sidebar behavior | `src/components/Layout.tsx` |
| Command palette | `src/components/CommandPalette.tsx`, `src/hooks/useHotkeys.ts` |
| Saved filters | `src/pages/FindingsPage.tsx`, `src/hooks/useRoute.ts`, `src/store/preferences.tsx` |
| Finding investigation | `src/components/FindingDrawer.tsx`, `src/pages/FindingsPage.tsx` |
| Scan timeline | `src/pages/ScanDetailPage.tsx`, `src/types.ts`, `src/api/` |
| Activity feed | `src/pages/OverviewPage.tsx`, `src/api/` |
| Posture score | `src/pages/OverviewPage.tsx`, `src/components/PostureScore.tsx`, `src/api/` |
| Repository risk profile | `src/pages/RepositoriesPage.tsx`, `src/api/` |
| Analytics filters | `src/pages/AnalyticsPage.tsx`, `src/components/FilterBar.tsx` |
| Notification preferences | `src/pages/SettingsPage.tsx`, `src/api/` |
| Exports and reports | `src/components/ExportMenu.tsx`, `src/api/` |
| Onboarding | `src/components/OnboardingTour.tsx`, `src/store/preferences.tsx` |

The exact file names may change if an existing helper already provides the same
responsibility. Reuse existing API clients, route helpers, resource loading, and
toast/error patterns.

---

## 12. Backend/API dependencies

The following can be implemented frontend-first:

- Theme presets.
- Density and sidebar preferences.
- Dashboard widget visibility/order.
- Command palette.
- Local saved filters.
- Onboarding.
- Accessibility improvements.
- Local export of data already loaded in the browser.

The following likely require API support:

- Finding lifecycle timeline.
- Bulk finding updates.
- Assignment and ownership.
- Scan stage and event stream.
- Activity feed.
- Posture score and historical trend.
- Repository risk profile fields.
- Analytics date/filter queries.
- Notification preferences.
- Server-generated reports.

Before adding each API dependency:

1. Confirm whether an existing endpoint already provides the needed data.
2. Extend the shared API types and client together.
3. Define loading, empty, stale, and error states.
4. Preserve authorization and masking behavior.
5. Add an integration or contract test where the response shape is new.

---

## 13. Testing and verification

### Automated checks

- `npm run build`
- TypeScript checks through the build script.
- Component tests for preference migration and fallback behavior.
- Tests for saved-filter serialization and URL synchronization.
- Tests for command-palette keyboard shortcuts.
- Tests for theme contrast tokens.
- Tests for export masking.

### Manual checks

- Light, dark, system, and every theme preset.
- English and at least one translated locale, then all supported locales before
  release.
- Desktop, tablet, and mobile widths.
- Keyboard-only navigation.
- Reduced-motion mode.
- High-contrast mode.
- API unavailable, empty database, partial API response, and failed mutation.
- Findings with masking enabled and disabled.
- Long-running scans, paused scans, failed scans, and completed scans.

### Security checks

- No plaintext credential enters a browser-persisted preference.
- No plaintext credential appears in URLs, command-palette search history,
  activity feed, exports, analytics labels, or telemetry.
- Browser-only cached data contains presentation preferences only.
- Error messages do not echo sensitive API response fields.

---

## 14. Recommended first milestone

The first implementation milestone should include:

1. Typed UI preferences store.
2. Theme presets: Paper, Midnight, Ocean, Signal, Minimal, High contrast.
3. Light/dark/system mode.
4. Compact/comfortable density.
5. Collapsible sidebar.
6. Appearance settings UI.
7. Reset preferences action.
8. Shared translation keys.
9. Accessibility and reduced-motion handling.

This milestone is visually noticeable, mostly frontend-only, low risk to the
backend, and creates the foundation for dashboard customization and the command
palette.

The second milestone should focus on saved findings filters, bulk triage, and
the finding investigation drawer because those features directly improve the
security workflow.

---

## 15. Implementation status

Everything the plan listed as implementable frontend-first has been built, plus
the parts of the later phases the existing API can already support honestly. The
features that need server work are listed at the end with what is missing, and
are *not* faked in the interface.

### Shipped

| Area | Where |
|---|---|
| Typed preference model, validation, `gs-mode` migration, reset | `src/store/preferences.tsx` |
| Six theme presets × light/dark, density, reduced motion, focus ring | `src/theme.ts`, `src/store/mode.tsx` |
| Appearance settings with live preset previews | `src/pages/SettingsPage.tsx` |
| Collapsible sidebar, mobile bottom navigation, "more" drawer | `src/components/Layout.tsx` |
| Dashboard widgets: show, hide, reorder, four layout presets | `src/pages/OverviewPage.tsx`, `src/components/DashboardWidget.tsx` |
| Command palette (`Ctrl/⌘+K`), `N`, `/`, `Ctrl/⌘+[` | `src/components/CommandPalette.tsx`, `src/hooks/useHotkeys.ts` |
| Filter presets, saved filters, URL synchronisation | `src/components/SavedFilterMenu.tsx`, `src/pages/FindingsPage.tsx` |
| Exposure and ordering refinements, with the window stated in the UI | `src/pages/FindingsPage.tsx` |
| Bulk triage with confirmation and reported partial failure | `src/components/BulkBar.tsx`, `src/hooks/useTriage.ts` |
| Finding drawer: why-detected, timeline, remediation, related, copy-safe actions | `src/components/FindingDrawer.tsx` |
| Scan stage timeline, honest ETA, completion summary | `src/components/ScanTimeline.tsx`, `src/pages/ScanDetailPage.tsx` |
| Activity feed from server timestamps only | `src/components/ActivityFeed.tsx` |
| Explainable posture score and per-repository risk | `src/posture.ts`, `src/components/PostureScore.tsx` |
| Repository risk column, ordering, recommended next step | `src/pages/RepositoriesPage.tsx` |
| Analytics cross-filters, click-through, per-figure CSV, top repositories, oldest open | `src/pages/AnalyticsPage.tsx` |
| CSV / JSON / incident / executive exports, masked, scope-stamped | `src/export.ts`, `src/components/ExportMenu.tsx` |
| Seven-step onboarding, restartable from settings | `src/components/OnboardingTour.tsx` |
| unDraw illustrations, re-inked from the active theme | `src/components/Illustration.tsx`, `src/assets/undraw/` |
| Skeletons, illustrated empty states, accessible labels | `src/components/States.tsx` and every page |
| 250+ new strings in all seven languages | `src/i18n.ui.ts` |
| Tests: preferences, export masking, posture, stages, contrast, i18n coverage, render smoke | `frontend/test/` |

`npm run check` (`tsc && vitest run && vite build`) is the gate; `make
frontend-check` runs it.

### Deferred, because the API cannot support it yet

| Feature | What the server would have to add |
|---|---|
| Finding lifecycle history | Status-change events per finding; the drawer timeline currently shows commit, detection, probe and current status |
| Bulk update in one call | A batch `PATCH /v1/findings`; bulk triage fans out and reports partial failure |
| Owner assignment | An assignee field on a finding |
| Server-side exposure filter and ordering | `reachable_from_head`, `probe_status` and a sort key on `GET /v1/findings`; today the refinement is applied to one 500-row window and says so |
| Analytics date range and period comparison | Range parameters on `GET /v1/analytics` |
| Posture trend ("↑ 8 this month") | A historical series; the score is shown without a trend rather than with a fabricated one |
| Per-severity, digest and per-repository notification preferences | Fields on `AppSettings`; only the master switch exists |
| Per-repository scan failures and retry | Per-repository state on a scan job |
| Live event stream | Server-sent events or a websocket; the activity feed is assembled from timestamps |
| Server-generated PDF reports, report history | A report service |
| Team-wide saved filters | A saved-filter endpoint; filters are per-browser |
