import Card from '@mui/material/Card'
import Box from '@mui/material/Box'
import Markdown from '../components/Markdown'
import { PageHeader } from '../components/Layout'
import { useT } from '../i18n'
import guideEn from '../docs/guide.en.md?raw'
import guideVi from '../docs/guide.vi.md?raw'

// The usage guide is a bundled Markdown file per language; it switches with the
// EN/VI toggle like the rest of the UI.
export default function HelpPage() {
  const { t, lang } = useT()
  return (
    <>
      <PageHeader title={t('help.title')} subtitle={t('help.subtitle')} />
      <Card sx={{ p: { xs: 2, md: 4 }, maxWidth: 860 }}>
        <Box sx={{ mx: 'auto' }}>
          <Markdown source={lang === 'vi' ? guideVi : guideEn} />
        </Box>
      </Card>
    </>
  )
}
