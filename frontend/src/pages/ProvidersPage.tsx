import Box from '@mui/material/Box'
import Card from '@mui/material/Card'
import { PageHeader } from '../components/Layout'
import SeverityChip from '../components/SeverityChip'
import { Empty, LoadError, Loading } from '../components/States'
import { useResource } from '../hooks/useResource'
import { CATEGORY_ORDER, providerMeta } from '../providers'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import type { Provider } from '../types'

export default function ProvidersPage() {
  const { api } = useSettings()
  const { t } = useT()
  const res = useResource(() => api.providers(), [api], 60000)

  if (res.loading) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const providers = res.data!.providers

  // Group by display category, keep providers alphabetical by label within each.
  const groups = new Map<string, Provider[]>()
  for (const p of providers) {
    const cat = providerMeta(p.slug).category
    ;(groups.get(cat) ?? groups.set(cat, []).get(cat)!).push(p)
  }
  const order = [
    ...CATEGORY_ORDER.filter(c => groups.has(c)),
    ...[...groups.keys()].filter(c => !CATEGORY_ORDER.includes(c as never)).sort(),
  ]
  const totalRules = providers.reduce((n, p) => n + p.rules, 0)

  return (
    <>
      <PageHeader
        title={t('providers.title')}
        subtitle={t('providers.subtitle')}
        actions={
          <Box sx={{ color: 'text.secondary', fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap' }}>
            {t('providers.count', { providers: providers.length, rules: totalRules })}
          </Box>
        }
      />
      {!providers.length ? (
        <Card><Empty art="security" size={200} title={t('providers.empty')} /></Card>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {order.map(cat => {
            const items = [...groups.get(cat)!].sort((a, b) =>
              providerMeta(a.slug).label.localeCompare(providerMeta(b.slug).label))
            return (
              <Box key={cat}>
                <Box sx={{
                  display: 'flex', alignItems: 'baseline', gap: 1, mb: 1.25,
                  color: 'text.secondary', fontSize: 12, fontWeight: 700,
                  textTransform: 'uppercase', letterSpacing: '.06em',
                }}>
                  {cat}
                  <Box component="span" sx={{ color: 'text.disabled', fontWeight: 600 }}>· {items.length}</Box>
                </Box>
                <Box sx={{
                  display: 'grid', gap: 1.25,
                  gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: '1fr 1fr 1fr' },
                }}>
                  {items.map(p => <ProviderCard key={p.slug} provider={p} />)}
                </Box>
              </Box>
            )
          })}
        </Box>
      )}
    </>
  )
}

function ProviderCard({ provider }: { provider: Provider }) {
  const { t } = useT()
  const meta = providerMeta(provider.slug)
  return (
    <Card variant="outlined" sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1.5 }}>
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Box sx={{ fontWeight: 700, fontSize: 14, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {meta.label}
        </Box>
        <Box sx={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: 11.5, color: 'text.secondary', mt: .25 }}>
          {provider.slug} · {t('providers.rules', { n: provider.rules })}
        </Box>
      </Box>
      <SeverityChip severity={provider.severity} />
    </Card>
  )
}
