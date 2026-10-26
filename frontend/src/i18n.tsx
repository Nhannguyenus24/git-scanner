import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'

// A dependency-free i18n: a flat key -> {en, vi} dictionary and a t() lookup.
// A missing key returns itself, so partial coverage degrades to English-ish
// labels rather than blank space. t('x', {n: 3}) fills {n} placeholders.

export type Lang = 'en' | 'vi'

type Entry = { en: string; vi: string }
const DICT: Record<string, Entry> = {
  'brand.tagline':        { en: 'secret scanner', vi: 'máy quét bí mật' },

  'nav.overview':         { en: 'Overview', vi: 'Tổng quan' },
  'nav.analytics':        { en: 'Analytics', vi: 'Phân tích' },
  'nav.scans':            { en: 'Scans', vi: 'Lượt quét' },
  'nav.findings':         { en: 'Findings', vi: 'Phát hiện' },
  'nav.repositories':     { en: 'Repositories', vi: 'Kho mã' },
  'nav.candidates':       { en: 'Unmapped', vi: 'Chưa gắn' },
  'nav.help':             { en: 'Help', vi: 'Trợ giúp' },
  'help.title':           { en: 'Help & usage guide', vi: 'Trợ giúp & hướng dẫn' },
  'help.subtitle':        { en: 'How to use the GitScanner dashboard', vi: 'Cách dùng bảng điều khiển GitScanner' },
  'nav.newScan':          { en: 'New scan', vi: 'Quét mới' },
  'nav.scan':             { en: 'Scan', vi: 'Quét' },
  'toast.scanQueued':     { en: 'Scan queued — {kind}: {value}', vi: 'Đã xếp hàng quét — {kind}: {value}' },

  'api.checking':         { en: 'Checking API…', vi: 'Đang kiểm tra API…' },
  'api.ok':               { en: 'API connected', vi: 'Đã kết nối API' },
  'api.error':            { en: 'API unreachable', vi: 'Không tới được API' },
  'mode.toLight':         { en: 'Switch to light paper', vi: 'Chuyển nền giấy sáng' },
  'mode.toDark':          { en: 'Switch to dark paper', vi: 'Chuyển nền giấy tối' },
  'lang.switch':          { en: 'Tiếng Việt', vi: 'English' },

  'common.retry':         { en: 'Retry', vi: 'Thử lại' },
  'common.clearFilters':  { en: 'Clear filters', vi: 'Xóa bộ lọc' },
  'common.any':           { en: 'Any', vi: 'Tất cả' },
  'common.all':           { en: 'All', vi: 'Tất cả' },
  'common.active':        { en: 'Active', vi: 'Đang chạy' },
  'common.finished':      { en: 'Finished', vi: 'Đã xong' },
  'common.showAll':       { en: 'Show all', vi: 'Hiện tất cả' },
  'common.startScan':     { en: 'Start a scan', vi: 'Bắt đầu quét' },
  'common.cancel':        { en: 'Cancel', vi: 'Hủy' },
  'common.loadError':     { en: 'Could not reach the API: {error}. Check the address in the top bar and that the stack is up.', vi: 'Không tới được API: {error}. Kiểm tra địa chỉ ở thanh trên và xem stack đã chạy chưa.' },

  'sev.critical': { en: 'critical', vi: 'nghiêm trọng' },
  'sev.high':     { en: 'high', vi: 'cao' },
  'sev.medium':   { en: 'medium', vi: 'trung bình' },
  'sev.low':      { en: 'low', vi: 'thấp' },
  'sev.info':     { en: 'info', vi: 'thông tin' },

  'state.queued':    { en: 'queued', vi: 'chờ' },
  'state.running':   { en: 'running', vi: 'đang chạy' },
  'state.paused':    { en: 'paused', vi: 'tạm dừng' },
  'state.completed': { en: 'completed', vi: 'hoàn tất' },
  'state.failed':    { en: 'failed', vi: 'lỗi' },
  'state.cancelled': { en: 'cancelled', vi: 'đã hủy' },

  'status.open':           { en: 'Open', vi: 'Đang mở' },
  'status.rotated':        { en: 'Rotated', vi: 'Đã xoay khóa' },
  'status.false_positive': { en: 'False positive', vi: 'Báo nhầm' },
  'status.ignored':        { en: 'Ignored', vi: 'Bỏ qua' },

  'overview.title':    { en: 'Overview', vi: 'Tổng quan' },
  'overview.subtitle': { en: 'Open exposure across every scanned repository', vi: 'Mức độ lộ đang mở trên toàn bộ kho đã quét' },
  'overview.openFindings':  { en: 'Open findings', vi: 'Phát hiện đang mở' },
  'overview.totalHint':     { en: '{n} total', vi: '{n} tổng' },
  'overview.critical':      { en: 'Critical', vi: 'Nghiêm trọng' },
  'overview.liveAtHead':    { en: 'Live at HEAD', vi: 'Còn ở HEAD' },
  'overview.liveHint':      { en: 'still in the current tree', vi: 'còn trong cây hiện tại' },
  'overview.exposedRepos':  { en: 'Exposed repos', vi: 'Kho bị lộ' },
  'overview.activeScans':   { en: 'Active scans', vi: 'Lượt quét đang chạy' },
  'overview.unmapped':      { en: 'Unmapped shapes', vi: 'Dạng chưa gắn' },
  'overview.unmappedHint':  { en: 'awaiting a provider', vi: 'chờ gắn nhà cung cấp' },
  'overview.bySeverity':    { en: 'Open by severity', vi: 'Mở theo mức độ' },
  'overview.byProvider':    { en: 'Open by provider', vi: 'Mở theo nhà cung cấp' },
  'overview.triage':        { en: 'Triage', vi: 'Phân loại' },
  'overview.recentScans':   { en: 'Recent scans', vi: 'Lượt quét gần đây' },
  'overview.allScans':      { en: 'All scans →', vi: 'Tất cả →' },
  'overview.reposOf':       { en: '{a}/{b} repos', vi: '{a}/{b} kho' },
  'overview.blobs':         { en: '{n} blobs', vi: '{n} blob' },
  'overview.found':         { en: '{n} found', vi: '{n} phát hiện' },
  'overview.emptyTitle':    { en: 'No scans yet', vi: 'Chưa có lượt quét' },
  'overview.emptyBody':     { en: 'Scan a repository, a user or an organisation. Results appear here as the pipeline works through them.', vi: 'Quét một kho, một người dùng hoặc một tổ chức. Kết quả sẽ hiện ở đây khi pipeline xử lý.' },

  'analytics.title':    { en: 'Analytics', vi: 'Phân tích' },
  'analytics.subtitle': { en: 'A read on the scan corpus — what was found, where it lives, and when it was introduced', vi: 'Bức tranh toàn bộ dữ liệu quét — tìm thấy gì, ở đâu, và đưa vào khi nào' },
  'analytics.findings':    { en: 'Findings', vi: 'Phát hiện' },
  'analytics.openOf':      { en: '{n} open', vi: '{n} đang mở' },
  'analytics.repositories':{ en: 'Repositories', vi: 'Kho mã' },
  'analytics.exposedOf':   { en: 'exposed / scanned', vi: 'bị lộ / đã quét' },
  'analytics.timeline':    { en: 'Leak introduction timeline', vi: 'Dòng thời gian rò rỉ' },
  'analytics.timelineCap': { en: 'Findings by the month their credential was committed', vi: 'Phát hiện theo tháng credential được commit' },
  'analytics.bySeverity':  { en: 'Open by severity', vi: 'Mở theo mức độ' },
  'analytics.byProvider':  { en: 'Open by provider', vi: 'Mở theo nhà cung cấp' },
  'analytics.byProviderCap':{ en: 'Named providers plus the unmapped catch-all', vi: 'Nhà cung cấp có tên và nhóm chưa gắn' },
  'analytics.byAppType':   { en: 'By application type', vi: 'Theo loại ứng dụng' },
  'analytics.triage':      { en: 'Triage status', vi: 'Trạng thái phân loại' },
  'analytics.rules':       { en: 'Detection rules', vi: 'Luật phát hiện' },
  'analytics.rulesCap':    { en: 'Which rules fire most across the corpus', vi: 'Luật nào kích hoạt nhiều nhất' },
  'analytics.liveTitle':   { en: 'Live credential validation', vi: 'Kiểm tra credential còn sống' },
  'analytics.liveCap':     { en: 'Results of the liveness probe', vi: 'Kết quả kiểm tra còn sống' },
  'analytics.liveCapOff':  { en: 'Enable PROBE_SECRETS to validate found keys', vi: 'Bật PROBE_SECRETS để kiểm tra key tìm được' },
  'analytics.liveEmpty':   { en: 'No credentials have been validated. With probing on, a confirmed-live key is flagged "rotate now".', vi: 'Chưa có credential nào được kiểm tra. Khi bật probe, key còn sống sẽ được gắn cờ "xoay khóa ngay".' },
  'analytics.open':        { en: 'open', vi: 'mở' },
  'analytics.checked':     { en: 'checked', vi: 'đã kiểm' },
  'probe.live':   { en: 'Live — rotate', vi: 'Còn sống — xoay khóa' },
  'probe.dead':   { en: 'Revoked', vi: 'Đã thu hồi' },
  'probe.unverified': { en: 'No prober', vi: 'Không có probe' },
  'probe.error':  { en: 'Check failed', vi: 'Kiểm tra lỗi' },

  'scans.title':      { en: 'Scans', vi: 'Lượt quét' },
  'scans.subtitle':   { en: 'Every job the API has accepted, newest first', vi: 'Mọi job API đã nhận, mới nhất trước' },
  'scans.col.state':    { en: 'State', vi: 'Trạng thái' },
  'scans.col.target':   { en: 'Target', vi: 'Mục tiêu' },
  'scans.col.progress': { en: 'Progress', vi: 'Tiến độ' },
  'scans.col.blobs':    { en: 'Blobs', vi: 'Blob' },
  'scans.col.newFindings': { en: 'New findings', vi: 'Phát hiện mới' },
  'scans.col.started':  { en: 'Started', vi: 'Bắt đầu' },
  'scans.col.duration': { en: 'Duration', vi: 'Thời lượng' },
  'scans.reposOf':      { en: '{a}/{b} repositories', vi: '{a}/{b} kho' },
  'scans.emptyActive':  { en: 'Nothing running', vi: 'Không có gì đang chạy' },
  'scans.empty':        { en: 'No scans yet', vi: 'Chưa có lượt quét' },

  'scan.pause':  { en: 'Pause', vi: 'Tạm dừng' },
  'scan.resume': { en: 'Resume', vi: 'Tiếp tục' },
  'scan.cancel': { en: 'Cancel', vi: 'Hủy' },
  'scan.rescan': { en: 'Rescan', vi: 'Quét lại' },
  'scan.delete': { en: 'Delete', vi: 'Xóa' },
  'scan.deleteConfirm': { en: 'Delete this scan and all findings it recorded? This cannot be undone.', vi: 'Xóa lượt quét này và mọi phát hiện nó ghi lại? Không thể hoàn tác.' },
  'scan.deleted':   { en: 'Scan deleted', vi: 'Đã xóa lượt quét' },
  'scan.rescanned': { en: 'Rescan queued', vi: 'Đã xếp hàng quét lại' },
  'footer.rights':  { en: 'For authorized security testing only', vi: 'Chỉ dùng cho kiểm thử bảo mật được ủy quyền' },
  'scan.submitted': { en: '{kind} scan · submitted {ago} · {dur}', vi: 'quét {kind} · gửi {ago} · {dur}' },
  'scan.reposCount': { en: 'Repositories', vi: 'Số kho' },
  'scan.blobsScanned': { en: 'Blobs scanned', vi: 'Blob đã quét' },
  'scan.newFindings': { en: 'New findings', vi: 'Phát hiện mới' },
  'scan.newFindingsHint': { en: 'first seen by this job', vi: 'lần đầu thấy bởi job này' },
  'scan.openInJob': { en: 'Open in this job', vi: 'Mở trong job này' },
  'scan.reposProgress': { en: '{a} of {b} repositories', vi: '{a}/{b} kho' },
  'scan.findingsTitle': { en: 'Findings recorded by this scan', vi: 'Phát hiện ghi bởi lượt quét này' },
  'scan.openInFindings': { en: 'Open in Findings →', vi: 'Mở ở trang Phát hiện →' },
  'scan.notFound': { en: 'Scan not found', vi: 'Không thấy lượt quét' },
  'scan.notFoundBody': { en: 'It may have been removed by a data wipe.', vi: 'Có thể đã bị xóa khi dọn dữ liệu.' },
  'scan.back': { en: 'Back to scans', vi: 'Về danh sách quét' },
  'scan.emptyActive': { en: 'Nothing yet — results appear as repositories finish.', vi: 'Chưa có gì — kết quả hiện khi từng kho xong.' },
  'scan.emptyDone': { en: 'This scan recorded no new findings. A credential already found by an earlier scan stays attributed to that one.', vi: 'Lượt quét này không ghi phát hiện mới. Credential đã tìm thấy ở lượt trước vẫn thuộc về lượt đó.' },

  'findings.title':    { en: 'Findings', vi: 'Phát hiện' },
  'findings.matching': { en: '{n} matching', vi: '{n} khớp' },
  'findings.subtitle': { en: 'Every credential the pipeline has recorded', vi: 'Mọi credential pipeline đã ghi nhận' },
  'findings.search':   { en: 'Search path, rule, provider or repository', vi: 'Tìm đường dẫn, luật, nhà cung cấp hoặc kho' },
  'findings.severity': { en: 'Severity', vi: 'Mức độ' },
  'findings.provider': { en: 'Provider', vi: 'Nhà cung cấp' },
  'findings.status':   { en: 'Status', vi: 'Trạng thái' },
  'findings.oneRepo':  { en: 'One repository', vi: 'Một kho' },
  'findings.col.severity':   { en: 'Severity', vi: 'Mức độ' },
  'findings.col.credential': { en: 'Credential', vi: 'Credential' },
  'findings.col.repository': { en: 'Repository', vi: 'Kho' },
  'findings.col.location':   { en: 'Location', vi: 'Vị trí' },
  'findings.col.exposure':   { en: 'Exposure', vi: 'Mức lộ' },
  'findings.col.status':     { en: 'Status', vi: 'Trạng thái' },
  'findings.col.detected':   { en: 'Detected', vi: 'Phát hiện lúc' },
  'findings.unmapped':  { en: 'unmapped', vi: 'chưa gắn' },
  'findings.atHead':    { en: 'at HEAD', vi: 'ở HEAD' },
  'findings.history':   { en: 'history', vi: 'lịch sử' },
  'findings.atHeadTip': { en: 'Still present at HEAD', vi: 'Vẫn còn ở HEAD' },
  'findings.historyTip':{ en: 'Only in history — removed from the current tree but still clonable', vi: 'Chỉ trong lịch sử — đã xóa khỏi cây hiện tại nhưng vẫn clone được' },
  'findings.commitUnknown': { en: 'commit unknown — rescan to attribute', vi: 'chưa rõ commit — quét lại để gắn' },
  'findings.markAs':    { en: 'Mark as', vi: 'Đánh dấu là' },
  'findings.empty':     { en: 'No findings match.', vi: 'Không có phát hiện nào khớp.' },
  'findings.probeLiveTip':   { en: 'Live — authenticated as {who}. Rotate now.', vi: 'Còn sống — xác thực là {who}. Xoay khóa ngay.' },
  'findings.probeLiveTipNo': { en: 'Live — this credential still works. Rotate now.', vi: 'Còn sống — credential này vẫn dùng được. Xoay khóa ngay.' },
  'findings.probeDeadTip':   { en: 'The provider rejected this credential — already rotated or revoked.', vi: 'Nhà cung cấp từ chối credential này — đã xoay khóa hoặc thu hồi.' },
  'findings.probeOtherTip':  { en: 'Liveness check could not complete (rate limit or network).', vi: 'Không kiểm tra được còn sống (giới hạn tần suất hoặc mạng).' },

  'repos.title':    { en: 'Repositories', vi: 'Kho mã' },
  'repos.subtitle': { en: '{a} of {b} have open findings', vi: '{a}/{b} kho có phát hiện đang mở' },
  'repos.filter':   { en: 'Filter by owner/name', vi: 'Lọc theo chủ/tên' },
  'repos.onlyOpen': { en: 'Only with open findings', vi: 'Chỉ kho có phát hiện mở' },
  'repos.col.repository': { en: 'Repository', vi: 'Kho' },
  'repos.col.type':       { en: 'Type', vi: 'Loại' },
  'repos.col.critical':   { en: 'Critical', vi: 'Nghiêm trọng' },
  'repos.col.high':       { en: 'High', vi: 'Cao' },
  'repos.col.medium':     { en: 'Medium', vi: 'Trung bình' },
  'repos.col.low':        { en: 'Low', vi: 'Thấp' },
  'repos.col.openTotal':  { en: 'Open / total', vi: 'Mở / tổng' },
  'repos.col.lastScanned':{ en: 'Last scanned', vi: 'Quét lần cuối' },
  'repos.empty':    { en: 'No repositories match', vi: 'Không có kho nào khớp' },

  'cand.title':    { en: 'Unmapped credential shapes', vi: 'Các dạng credential chưa gắn' },
  'cand.subtitle': { en: 'High-entropy secrets no named rule claims, clustered by shape. A frequent shape is a provider worth a rule.', vi: 'Các bí mật entropy cao chưa luật nào nhận, gom theo hình dạng. Dạng xuất hiện nhiều là nhà cung cấp đáng viết luật.' },
  'cand.col.name':    { en: 'Guessed name', vi: 'Tên đoán' },
  'cand.col.sample':  { en: 'Sample', vi: 'Mẫu' },
  'cand.col.shape':   { en: 'Shape', vi: 'Hình dạng' },
  'cand.col.context': { en: 'Context', vi: 'Ngữ cảnh' },
  'cand.col.seen':    { en: 'Seen', vi: 'Số lần' },
  'cand.col.status':  { en: 'Status', vi: 'Trạng thái' },
  'cand.col.lastSeen':{ en: 'Last seen', vi: 'Lần cuối' },
  'cand.unknown':   { en: 'unknown', vi: 'chưa rõ' },
  'cand.chars':     { en: '{n} chars', vi: '{n} ký tự' },
  'cand.empty':     { en: 'Backlog is empty', vi: 'Không có tồn đọng' },
  'cand.emptyBody': { en: 'Every credential found so far was attributed to a known provider.', vi: 'Mọi credential tới nay đều đã gắn được nhà cung cấp.' },

  'dialog.title':     { en: 'New Scan', vi: 'Quét mới' },
  'dialog.repo':      { en: 'Repository', vi: 'Kho' },
  'dialog.user':      { en: 'User', vi: 'Người dùng' },
  'dialog.org':       { en: 'Org', vi: 'Tổ chức' },
  'dialog.random':    { en: 'Random', vi: 'Ngẫu nhiên' },
  'dialog.cloneUrl':  { en: 'Clone URL (HTTPS)', vi: 'URL clone (HTTPS)' },
  'dialog.login':     { en: 'GitHub login', vi: 'Tài khoản GitHub' },
  'dialog.includeForks': { en: 'Include forked repositories', vi: 'Gồm cả kho fork' },
  'dialog.required':  { en: 'This field is required', vi: 'Bắt buộc nhập' },
  'dialog.start':     { en: 'Start Scan', vi: 'Bắt đầu quét' },
  'dialog.help.repo': { en: 'Must be an HTTPS clone URL (no SSH, no credentials in the URL)', vi: 'Phải là URL clone HTTPS (không SSH, không chứa credential trong URL)' },
  'dialog.help.user': { en: 'Alphanumeric + hyphens, 1–39 characters', vi: 'Chữ, số và gạch nối, 1–39 ký tự' },
  'dialog.help.org':  { en: 'Alphanumeric + hyphens, 1–39 characters', vi: 'Chữ, số và gạch nối, 1–39 ký tự' },
  'dialog.count':     { en: 'Number of repositories', vi: 'Số lượng kho' },
  'dialog.forever':   { en: 'Run forever (until paused or cancelled)', vi: 'Chạy mãi (đến khi tạm dừng/huỷ)' },
  'dialog.countInvalid': { en: 'Enter a positive whole number', vi: 'Nhập số nguyên dương' },
  'dialog.help.random': { en: 'Randomly sampled public repos — for testing detection only', vi: 'Kho công khai lấy ngẫu nhiên — chỉ để kiểm thử dò quét' },
}

function lookup(key: string, lang: Lang, vars?: Record<string, string | number>): string {
  const entry = DICT[key]
  let s = entry ? entry[lang] : key
  if (vars) for (const [k, v] of Object.entries(vars)) s = s.replace(`{${k}}`, String(v))
  return s
}

interface Ctx { lang: Lang; setLang: (l: Lang) => void; t: (key: string, vars?: Record<string, string | number>) => string }
const I18nCtx = createContext<Ctx | null>(null)

function initialLang(): Lang {
  try {
    const saved = localStorage.getItem('gs-lang')
    if (saved === 'en' || saved === 'vi') return saved
  } catch { /* ignore */ }
  return typeof navigator !== 'undefined' && navigator.language?.toLowerCase().startsWith('vi') ? 'vi' : 'en'
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLang] = useState<Lang>(initialLang)
  useEffect(() => { try { localStorage.setItem('gs-lang', lang) } catch { /* ignore */ } }, [lang])
  const value = useMemo<Ctx>(() => ({ lang, setLang, t: (k, v) => lookup(k, lang, v) }), [lang])
  return <I18nCtx.Provider value={value}>{children}</I18nCtx.Provider>
}

export function useT(): Ctx {
  const ctx = useContext(I18nCtx)
  if (!ctx) throw new Error('useT must be inside I18nProvider')
  return ctx
}
