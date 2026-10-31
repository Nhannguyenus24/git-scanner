package api

import (
	"net/http"

	"github.com/mservice/gitscanner/internal/store"
)

// pageParam reads ?limit= and ?offset=. Absent or invalid values fall back to
// the store's defaults; the store clamps whatever is asked for.
func pageParam(r *http.Request) store.Page {
	return store.Page{Limit: intParam(r, "limit"), Offset: intParam(r, "offset")}
}

// paged writes a list response in the shape every paged route shares: the
// items under their own key, the total that matched, and the limit and offset
// actually applied — which can differ from what was asked when it was out of
// range, so a client pages from what it got rather than what it sent.
func paged(w http.ResponseWriter, key string, items any, total int, p store.Page) {
	limit, offset := p.Bounds()
	writeJSON(w, http.StatusOK, map[string]any{key: items, "total": total, "limit": limit, "offset": offset})
}
