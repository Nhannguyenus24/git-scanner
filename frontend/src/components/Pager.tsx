import TablePagination from '@mui/material/TablePagination'
import { num } from '../format'
import { useT } from '../i18n'

// The page control every paged table shares, so the labels are translated
// once and the size options are the same everywhere. Page sizes stay within
// the server's cap of 500 rows a request.
export const PAGE_SIZES = [25, 50, 100, 200]

interface Props {
  total:   number
  page:    number
  size:    number
  onPage:  (page: number) => void
  onSize:  (size: number) => void
}

export default function Pager({ total, page, size, onPage, onSize }: Props) {
  const { t } = useT()
  return (
    <TablePagination component="div" count={total} page={Math.min(page, Math.max(0, Math.ceil(total / size) - 1))}
      rowsPerPage={size} rowsPerPageOptions={PAGE_SIZES}
      labelRowsPerPage={t('pager.size')}
      labelDisplayedRows={({ from, to, count }) => t('pager.range', { from: num(from), to: num(to), total: num(count) })}
      getItemAriaLabel={type => t(`pager.${type}`)}
      showFirstButton showLastButton
      onPageChange={(_, n) => onPage(n)}
      onRowsPerPageChange={e => onSize(Number(e.target.value))} />
  )
}
