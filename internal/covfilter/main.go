// Command covfilter strips known-integration-only lines out of a Go
// coverage profile (the plain-text format `go test -coverprofile` writes)
// before `go tool cover -func`/`-html` or Codecov ever compute a percentage
// from it. Go has no source-level "ignore this line" coverage pragma —
// unlike awk/grep-based filtering (which needs a POSIX shell environment
// not guaranteed on every Windows machine running `task coverage`), this is
// a plain Go program, so it works identically wherever `go` itself runs.
//
// Usage: go run ./internal/covfilter <profile-file>
//
// Currently hardcodes exactly one exclusion: internal/dialecttest/
// locking.go from line 46 onward — lockedFind's body and the lock-strength
// subtests that call it only execute when the real adapter under test
// supports that strength; under `-short` (no Docker), only driver/sqlite's
// untagged conformance test reaches this function, and every one of
// sqlite's Capabilities.Locking fields is correctly false. See
// locking.go's own "COVERAGE-EXCLUDED" comment and .specs/project/STATE.md
// AD-044 for the full reasoning. If locking.go's line numbers ever shift,
// update excludeFromLine below to match.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/leandroluk/golem/internal/must"
)

const (
	excludeFile     = "github.com/leandroluk/golem/internal/dialecttest/locking.go"
	excludeFromLine = 46
)

// osExit/stderr are indirected through package vars (not called/referenced
// directly) so tests can call main() in-process — swapping in a fake that
// records the code instead of terminating the test binary — and still have
// go test's coverage instrumentation count main()'s own lines as executed,
// which a subprocess-based test (a second, separately-compiled process)
// could not do.
var (
	osExit           = os.Exit
	stderr io.Writer = os.Stderr
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(stderr, "usage: covfilter <profile-file>")
		osExit(2)
		return
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(stderr, "covfilter:", err)
		osExit(1)
		return
	}
}

// run rewrites the profile at path in place, filtering it through
// filterLines. The whole file is read into memory first, then closed,
// before path is ever opened for writing — os.Create truncates on the same
// path os.Open just read from, so writing (even line-by-line) before
// reading is fully drained would race the read against the truncation.
// Split from filterLines so the actual filtering logic is testable with
// plain io.Reader/io.Writer values, no filesystem needed.
//
// io.ReadAll's and os.Create's own error branches are wrapped via
// internal/must (forcing "os.Open on path succeeds, but a subsequent
// read/re-create of that exact same path fails" needs an OS/filesystem
// permission model that behaves too differently across Windows/macOS/Linux
// to assert on portably in a unit test — see driver/*'s identical reasoning,
// STATE.md AD-043) — defer must.Recover(&err) converts any panic back into
// this function's normal error return before it can escape to main.
func run(path string) (err error) {
	defer must.Recover(&err)

	f, openErr := os.Open(path)
	if openErr != nil {
		return openErr
	}
	data := must.Value(io.ReadAll(f))
	f.Close()

	out := must.Value(os.Create(path))
	defer out.Close()

	return filterLines(bytes.NewReader(data), out)
}

// filterLines copies every line from r to w, dropping ones shouldExclude
// flags. Buffered at 10MB/line to comfortably fit any real coverage profile
// line (they're short — one file:range plus 2 small integers).
func filterLines(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	bw := bufio.NewWriter(w)
	for scanner.Scan() {
		line := scanner.Text()
		if shouldExclude(line) {
			continue
		}
		if _, err := bw.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// shouldExclude reports whether a coverage profile line belongs to
// excludeFile at or after excludeFromLine. Profile lines look like:
//
//	github.com/leandroluk/golem/internal/dialecttest/locking.go:46.2,46.71 1 1
//
// (the "mode: atomic" header line, and every other file's lines, never
// match the prefix check and are always kept.)
func shouldExclude(line string) bool {
	prefix := excludeFile + ":"
	if !strings.HasPrefix(line, prefix) {
		return false
	}
	rest := strings.TrimPrefix(line, prefix)
	dot := strings.IndexByte(rest, '.')
	if dot < 0 {
		return false
	}
	n, err := strconv.Atoi(rest[:dot])
	if err != nil {
		return false
	}
	return n >= excludeFromLine
}
