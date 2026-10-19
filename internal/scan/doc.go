// Package scan is the scan engine (plan.md §4): clone, blob walk, detection.
//
// Phase 0 holds only the Redis-backed state the engine and the api share.
//
// Files:
//
//	blobindex.go  which blob OIDs a repository has already had scanned
//	control.go    pause flags and resume cursors for scan jobs
package scan

// Redis key prefixes owned by this package. Kept together so a key collision
// with another package is visible in one place.
const (
	keyBlobs  = "blobs:"
	keyCursor = "cursor:"
	keyPause  = "pause:"
)
