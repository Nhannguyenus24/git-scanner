import Card from '@mui/material/Card'
import Box from '@mui/material/Box'
import Markdown from '../components/Markdown'
import { PageHeader } from '../components/Layout'
import { useT, type Lang } from '../i18n'
import guideEn from '../docs/guide.en.md?raw'
import guideVi from '../docs/guide.vi.md?raw'
import guideIt from '../docs/guide.it.md?raw'
import guideZh from '../docs/guide.zh.md?raw'
import guideEs from '../docs/guide.es.md?raw'
import guideDe from '../docs/guide.de.md?raw'
import guideFr from '../docs/guide.fr.md?raw'

const GUIDES: Record<Lang, string> = {
  en: guideEn,
  vi: guideVi,
  it: guideIt,
  zh: guideZh,
  es: guideEs,
  de: guideDe,
  fr: guideFr,
}

// The usage guide is a bundled Markdown file per language; it switches with the
// language selector like the rest of the UI.
export default function HelpPage() {
  const { t, lang } = useT()
  return (
    <>
      <PageHeader title={t('help.title')} subtitle={t('help.subtitle')} />
      <Card sx={{ p: { xs: 2, md: 4 }, maxWidth: 860 }}>
        <Box sx={{ mx: 'auto' }}>
          <Markdown source={GUIDES[lang] ?? guideEn} />
        </Box>
      </Card>
    </>
  )
}
