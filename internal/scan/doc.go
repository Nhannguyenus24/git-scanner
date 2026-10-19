// Package scan is the scan engine (plan.md §4): clone, blob walk, detection.
//
// Phase 0 holds the Redis-backed state the engine and the api share, and the
// first false-positive filter.
//
// Files:
//
//	blobindex.go  which blob OIDs a repository has already had scanned
//	control.go    pause flags and resume cursors for scan jobs
//	localfilter.go  skip credentials that only point at the local machine
package scan

// Redis key prefixes owned by this package. Kept together so a key collision
// with another package is visible in one place.
const (
	keyBlobs  = "blobs:"
	keyCursor = "cursor:"
	keyPause  = "pause:"
)
