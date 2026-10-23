package scan

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Blob is one object from a repository's history.
type Blob struct {
	OID  string
	Path string
	Data []byte
}

// Repo is the read-only view of a cloned repository the engine needs. Keeping
// it an interface (plan.md §1) means the git binary stays swappable and the
// engine is testable with an in-memory fake.
type Repo interface {
	// Blobs streams every distinct blob reachable in history to fn. Returning an
	// error from fn stops the walk and is propagated.
	Blobs(ctx context.Context, fn func(Blob) error) error
	// HeadOIDs returns the set of blob OIDs reachable from the default branch
	// tip, so a finding can be marked as live rather than only historical.
	HeadOIDs(ctx context.Context) (map[string]struct{}, error)
	// Close removes the working directory.
	Close() error
}

// Cloner produces a Repo from a clone URL.
type Cloner interface {
	Clone(ctx context.Context, cloneURL string) (Repo, error)
}

// gitEnv neutralises git's extension points before any untrusted repository is
// touched, matching the container hardening in plan.md §13.3. A submodule or
// alternate-transport URL in a hostile repo cannot run a command or fetch an
// attacker-chosen host when these are set.
//
// allowProtocols names transports permitted in addition to https. It is empty
// in production — only https clones — and exists so integration tests can clone
// a local file:// repository without weakening the default.
func gitEnv(allowProtocols []string) []string {
	// Start locked down: deny every protocol, then re-enable https and anything
	// explicitly allowed.
	pairs := [][2]string{
		{"core.hooksPath", "/dev/null"},
		{"protocol.allow", "never"},
		{"protocol.https.allow", "always"},
	}
	for _, p := range allowProtocols {
		pairs = append(pairs, [2]string{"protocol." + p + ".allow", "always"})
	}

	env := append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		fmt.Sprintf("GIT_CONFIG_COUNT=%d", len(pairs)),
	)
	for i, p := range pairs {
		env = append(env,
			fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", i, p[0]),
			fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", i, p[1]))
	}
	return env
}

// GitCLI clones by shelling out to the git binary into scratch directories.
type GitCLI struct {
	// ScratchDir is the parent for per-job mirror clones. Empty uses the OS temp
	// dir; in Kubernetes it is the size-limited /scratch volume (plan.md §13.3).
	ScratchDir string
	// MaxBlobBytes skips blobs larger than this; a 10 MB default keeps a vendored
	// binary or minified bundle from dominating a scan.
	MaxBlobBytes int
	// AllowProtocols permits transports beyond https. Leave empty in production;
	// integration tests set it to clone a local file:// repository.
	AllowProtocols []string
}

// NewGitCLI returns a cloner with sensible defaults.
func NewGitCLI(scratchDir string) *GitCLI {
	return &GitCLI{ScratchDir: scratchDir, MaxBlobBytes: 10 << 20}
}

// Clone makes a bare clone carrying every branch and tag, which is what lets
// the blob walk see the whole history rather than one branch.
//
// Deliberately --bare and not --mirror. A mirror clone fetches refs/*, and on
// GitHub that includes refs/pull/* — one ref per pull request ever opened. On a
// widely forked repository that is the overwhelming majority of its refs
// (octocat/Spoon-Knife advertises 63,775, of which 3 are branches), so a mirror
// pulls gigabytes of commits belonging to other people's forks. That is slow
// enough to look like a hang, it can exhaust the scratch volume, and the
// commits are not the target's history at all: scanning them would attribute
// another repository's secrets to this one, over a ref nobody authorized as a
// target (plan.md §0.1). --bare fetches branches and tags only.
func (g *GitCLI) Clone(ctx context.Context, cloneURL string) (Repo, error) {
	dir, err := os.MkdirTemp(g.ScratchDir, "scan-*")
	if err != nil {
		return nil, fmt.Errorf("scan: scratch dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--bare", "--quiet",
		"--no-recurse-submodules", cloneURL, dir)
	cmd.Env = gitEnv(g.AllowProtocols)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("scan: git clone: %w: %s", err, strings.TrimSpace(string(out)))
	}

	maxBlob := g.MaxBlobBytes
	if maxBlob <= 0 {
		maxBlob = 10 << 20
	}
	return &gitRepo{dir: dir, maxBlob: maxBlob, env: gitEnv(g.AllowProtocols)}, nil
}

type gitRepo struct {
	dir     string
	maxBlob int
	env     []string
}

func (r *gitRepo) Close() error { return os.RemoveAll(r.dir) }

func (r *gitRepo) git(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = r.env
	return cmd
}

// Blobs walks every distinct blob in history. It lists (oid, path) pairs with
// rev-list, deduplicates OIDs so identical content is scanned once (plan.md
// §4.1: scan unique blobs, not commits), and streams contents through a single
// long-lived cat-file --batch process.
func (r *gitRepo) Blobs(ctx context.Context, fn func(Blob) error) error {
	pathOf, oids, err := r.listBlobs(ctx)
	if err != nil {
		return err
	}

	// cat-file gets its own cancellable context so that leaving this function
	// early can kill it.
	//
	// Without that, every early return below deadlocks. Once this loop stops
	// reading, git blocks writing into a stdout nobody drains; because git has
	// stopped reading, the feeder goroutine blocks writing into a full stdin;
	// and Wait then waits forever on a process that can never exit. Cancelling
	// first is what makes the exit path terminate.
	batchCtx, cancelBatch := context.WithCancel(ctx)

	batch := r.git(batchCtx, "cat-file", "--batch")
	stdin, err := batch.StdinPipe()
	if err != nil {
		cancelBatch()
		return fmt.Errorf("scan: cat-file stdin: %w", err)
	}
	stdout, err := batch.StdoutPipe()
	if err != nil {
		cancelBatch()
		return fmt.Errorf("scan: cat-file stdout: %w", err)
	}
	if err := batch.Start(); err != nil {
		cancelBatch()
		return fmt.Errorf("scan: start cat-file: %w", err)
	}
	defer func() {
		cancelBatch()
		_ = batch.Wait()
	}()

	out := bufio.NewReaderSize(stdout, 1<<16)
	// Feed requests from a goroutine so writing to stdin and reading stdout do
	// not deadlock on a full pipe.
	writeErr := make(chan error, 1)
	go func() {
		defer func() { _ = stdin.Close() }()
		for _, oid := range oids {
			if _, err := fmt.Fprintln(stdin, oid); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- nil
	}()

	for _, oid := range oids {
		blob, err := readBatchObject(out, oid, pathOf[oid], r.maxBlob)
		if err != nil {
			return err
		}
		if blob == nil { // skipped (too large or not a blob)
			continue
		}
		if err := fn(*blob); err != nil {
			return err
		}
	}

	if err := <-writeErr; err != nil {
		return fmt.Errorf("scan: feed cat-file: %w", err)
	}
	return nil
}

// listBlobs returns a path for each OID and the deduplicated OID order.
func (r *gitRepo) listBlobs(ctx context.Context) (map[string]string, []string, error) {
	cmd := r.git(ctx, "rev-list", "--objects", "--all")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("scan: rev-list stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("scan: start rev-list: %w", err)
	}

	pathOf := map[string]string{}
	var oids []string
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		oid, path, _ := strings.Cut(sc.Text(), " ")
		if path == "" { // commits and trees have no path; only blobs carry one
			continue
		}
		if _, seen := pathOf[oid]; !seen {
			pathOf[oid] = path
			oids = append(oids, oid)
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, nil, fmt.Errorf("scan: rev-list: %w", err)
	}
	return pathOf, oids, sc.Err()
}

// readBatchObject reads one `cat-file --batch` record: a header line
// "<oid> <type> <size>\n", then <size> bytes, then a trailing newline.
func readBatchObject(out *bufio.Reader, oid, path string, maxBlob int) (*Blob, error) {
	header, err := out.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("scan: read cat-file header for %s: %w", oid, err)
	}
	fields := strings.Fields(strings.TrimSpace(header))
	if len(fields) != 3 {
		return nil, fmt.Errorf("scan: unexpected cat-file header %q", header)
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, fmt.Errorf("scan: bad object size %q: %w", fields[2], err)
	}

	// Always consume size+1 bytes (payload + trailing newline) so the stream
	// stays aligned even when we skip this blob.
	payload := make([]byte, size+1)
	if _, err := readFull(out, payload); err != nil {
		return nil, fmt.Errorf("scan: read object %s: %w", oid, err)
	}
	if fields[1] != "blob" || size > maxBlob {
		return nil, nil
	}
	return &Blob{OID: oid, Path: path, Data: payload[:size]}, nil
}

func readFull(out *bufio.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := out.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// HeadOIDs returns the blob OIDs reachable from HEAD, so a finding can be marked
// reachable-from-HEAD (live) rather than only present in history (plan.md §0.2).
func (r *gitRepo) HeadOIDs(ctx context.Context) (map[string]struct{}, error) {
	cmd := r.git(ctx, "ls-tree", "-r", "--format=%(objectname)", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		// A bare repo with an unborn HEAD has no tree; that is not an error, it
		// just means nothing is currently reachable.
		return map[string]struct{}{}, nil
	}
	set := map[string]struct{}{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			set[line] = struct{}{}
		}
	}
	return set, nil
}
