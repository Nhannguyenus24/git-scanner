package store

// Page is one window of a list the API serves: at most Limit rows, starting
// after Offset. Every list the dashboard can grow without bound takes one, and
// returns the total that matched so the caller can show where it is.
//
// A list that once returned everything is the failure this exists for: the
// repositories view sent 31 MB every ten seconds once a random scan had put
// 126k repositories in the table.
type Page struct {
	Limit  int
	Offset int
}

// Page sizes. The default is what a dashboard table shows; the cap keeps one
// request bounded whatever a caller asks for.
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// Bounds returns the limit and offset actually applied, for SQL and for the
// response that tells the caller what it got.
func (p Page) Bounds() (limit, offset int) {
	return clampLimit(p.Limit, DefaultPageSize, MaxPageSize), max(p.Offset, 0)
}
