package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldExclude(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"header", "mode: atomic", false},
		{"other file", "github.com/leandroluk/golem/internal/dialecttest/crud.go:21.0,25.0 1 1", false},
		{"target file before cutoff", "github.com/leandroluk/golem/internal/dialecttest/locking.go:45.99,46.2 1 1", false},
		{"target file at cutoff", "github.com/leandroluk/golem/internal/dialecttest/locking.go:46.2,46.71 1 1", true},
		{"target file after cutoff", "github.com/leandroluk/golem/internal/dialecttest/locking.go:98.3,98.87 1 0", true},
		{"target file no dot", "github.com/leandroluk/golem/internal/dialecttest/locking.go:46", false},
		{"target file non-numeric", "github.com/leandroluk/golem/internal/dialecttest/locking.go:abc.2,46.71 1 1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldExclude(tc.line); got != tc.want {
				t.Errorf("shouldExclude(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestFilterLines(t *testing.T) {
	in := "mode: atomic\n" +
		"github.com/leandroluk/golem/internal/dialecttest/crud.go:21.0,25.0 1 1\n" +
		"github.com/leandroluk/golem/internal/dialecttest/locking.go:46.2,46.71 1 0\n" +
		"github.com/leandroluk/golem/internal/dialecttest/locking.go:30.3,30.94 1 1\n"
	want := "mode: atomic\n" +
		"github.com/leandroluk/golem/internal/dialecttest/crud.go:21.0,25.0 1 1\n" +
		"github.com/leandroluk/golem/internal/dialecttest/locking.go:30.3,30.94 1 1\n"

	var out strings.Builder
	if err := filterLines(strings.NewReader(in), &out); err != nil {
		t.Fatalf("filterLines: %v", err)
	}
	if out.String() != want {
		t.Errorf("filtered = %q, want %q", out.String(), want)
	}
}

// errReader returns errAfter on Read once past its content, simulating a
// mid-stream I/O failure so scanner.Err()'s branch is reachable directly.
type errReader struct {
	content string
	read    bool
	err     error
}

func (r *errReader) Read(p []byte) (int, error) {
	if !r.read {
		r.read = true
		n := copy(p, r.content)
		return n, nil
	}
	return 0, r.err
}

func TestFilterLines_ScanError(t *testing.T) {
	r := &errReader{content: "mode: atomic\n", err: errors.New("boom")}
	var out strings.Builder
	err := filterLines(r, &out)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected scan error, got %v", err)
	}
}

// errWriter always fails, simulating a write failure (disk full, closed
// file, etc.) so filterLines' write-error branch is reachable directly.
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestFilterLines_WriteError(t *testing.T) {
	// bufio.Writer's default 4KB buffer means a single short WriteString
	// just buffers and returns nil — the underlying errWriter only gets
	// called (and fails) once a write no longer fits the remaining buffer,
	// so feed enough lines to force that overflow from inside WriteString
	// itself, not just via the trailing Flush.
	var sb strings.Builder
	for i := 0; i < 500; i++ { // 500*13 bytes > bufio.Writer's 4096-byte default buffer
		sb.WriteString("mode: atomic\n")
	}
	err := filterLines(strings.NewReader(sb.String()), errWriter{})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestRun_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coverage.txt")
	content := "mode: atomic\n" +
		"github.com/leandroluk/golem/internal/dialecttest/locking.go:46.2,46.71 1 0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := run(path); err != nil {
		t.Fatalf("run: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "mode: atomic\n" {
		t.Errorf("filtered content = %q", got)
	}
}

func TestRun_OpenError(t *testing.T) {
	if err := run(filepath.Join(t.TempDir(), "nonexistent.txt")); err == nil {
		t.Fatal("expected open error")
	}
}

// run's os.Create error branch (Open succeeds, Create on the same path
// fails) isn't exercised here: reliably forcing that split outcome needs an
// OS/filesystem permission model, which behaves too differently across
// Windows/macOS/Linux to assert on portably in a unit test. Accepted as an
// unreachable-in-practice defensive check, same class as this codebase's
// other documented exceptions (see driver/*'s equivalents) — filterLines'
// own write-error path (TestFilterLines_WriteError) is what actually proves
// this function's error-propagation logic works.

// withMainFakes swaps os.Args/osExit/stderr for the duration of fn, so
// main() can be called directly (in-process, no subprocess) — letting go
// test's coverage instrumentation count main()'s own lines as executed,
// which spawning a second, separately-compiled process could never do.
func withMainFakes(t *testing.T, args []string, fn func(exitCode func() int, stderrOut func() string)) {
	t.Helper()
	origArgs, origExit, origStderr := os.Args, osExit, stderr
	t.Cleanup(func() { os.Args = origArgs; osExit = origExit; stderr = origStderr })

	os.Args = args
	code := 0
	osExit = func(c int) { code = c }
	var buf strings.Builder
	stderr = &buf

	fn(func() int { return code }, buf.String)
}

func TestMain_UsageError(t *testing.T) {
	withMainFakes(t, []string{"covfilter"}, func(exitCode func() int, stderrOut func() string) {
		main()
		if exitCode() != 2 {
			t.Errorf("exit code = %d, want 2", exitCode())
		}
		if !strings.Contains(stderrOut(), "usage:") {
			t.Errorf("stderr = %q, want a usage message", stderrOut())
		}
	})
}

func TestMain_RunError(t *testing.T) {
	withMainFakes(t, []string{"covfilter", filepath.Join(t.TempDir(), "nonexistent.txt")}, func(exitCode func() int, stderrOut func() string) {
		main()
		if exitCode() != 1 {
			t.Errorf("exit code = %d, want 1", exitCode())
		}
		if !strings.Contains(stderrOut(), "covfilter:") {
			t.Errorf("stderr = %q, want a covfilter: error message", stderrOut())
		}
	})
}

func TestMain_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "coverage.txt")
	if err := os.WriteFile(path, []byte("mode: atomic\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	withMainFakes(t, []string{"covfilter", path}, func(exitCode func() int, stderrOut func() string) {
		main()
		if exitCode() != 0 {
			t.Errorf("exit code = %d, want 0 (osExit never called on success)", exitCode())
		}
		if stderrOut() != "" {
			t.Errorf("stderr = %q, want empty", stderrOut())
		}
	})
}
