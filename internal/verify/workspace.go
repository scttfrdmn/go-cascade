package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Runner owns the process-wide build environment shared by every candidate
// workspace. Sharing GOCACHE is what keeps the compile stages in the tens of
// milliseconds after the first build, which is the whole economic premise of a
// verification-saturated cascade.
type Runner struct {
	root        string   // scratch root, removed by Close
	goBin       string   // path to the go command
	env         []string // frozen child environment
	execWrapper []string // optional sandbox prefix for stages that run code
}

// NewRunner prepares the shared environment. goBin may be empty to use $PATH.
//
// The build cache lives outside the scratch root and survives the process. A
// per-run GOCACHE costs roughly 30s of cold compilation on the first candidate,
// which would dominate everything the router is trying to save; persisting it
// is what makes the measured warm numbers (build 43ms, vet 113ms, test 120ms)
// the ones that actually apply.
func NewRunner(goBin string, execWrapper []string) (*Runner, error) {
	if goBin == "" {
		p, err := exec.LookPath("go")
		if err != nil {
			return nil, fmt.Errorf("go toolchain not found on PATH: %w", err)
		}
		goBin = p
	}
	root, err := os.MkdirTemp("", "go-cascade-")
	if err != nil {
		return nil, err
	}
	persist := filepath.Join(os.TempDir(), "go-cascade-build")
	if ucd, err := os.UserCacheDir(); err == nil {
		persist = filepath.Join(ucd, "go-cascade", "build")
	}
	cache := filepath.Join(persist, "gocache")
	modcache := filepath.Join(persist, "modcache")
	for _, d := range []string{cache, modcache} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return &Runner{
		root:  root,
		goBin: goBin,
		env: []string{
			"HOME=" + root,
			"PATH=" + filepath.Dir(goBin) + string(os.PathListSeparator) + os.Getenv("PATH"),
			"GOCACHE=" + cache,
			"GOMODCACHE=" + modcache,
			"GOFLAGS=-mod=mod",
			// Standard-library-only generation means the module graph is empty,
			// so no proxy is needed and none is permitted.
			"GOPROXY=off",
			"GOTOOLCHAIN=local",
			"CGO_ENABLED=1", // required by -race
		},
		execWrapper: execWrapper,
	}, nil
}

// Close removes the scratch root.
func (r *Runner) Close() error { return os.RemoveAll(r.root) }

// Workspace is one candidate's module directory.
type Workspace struct {
	Dir string
	r   *Runner
}

// Files written into a workspace.
const (
	solutionFile = "solution.go"
	visibleFile  = "visible_test.go"
	hiddenFile   = "hidden_test.go"
)

// NewWorkspace materialises a candidate as a compilable module.
func (r *Runner) NewWorkspace(solution, visibleTests, hiddenTests string) (*Workspace, error) {
	dir, err := os.MkdirTemp(r.root, "cand-")
	if err != nil {
		return nil, err
	}
	gomod := "module gocascade/candidate\n\ngo 1.26\n"
	files := map[string]string{
		"go.mod":     gomod,
		solutionFile: solution,
	}
	if visibleTests != "" {
		files[visibleFile] = visibleTests
	}
	if hiddenTests != "" {
		files[hiddenFile] = hiddenTests
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return nil, err
		}
	}
	return &Workspace{Dir: dir, r: r}, nil
}

// Remove deletes the workspace.
func (w *Workspace) Remove() error { return os.RemoveAll(w.Dir) }

// SolutionPath is the path of the candidate source file.
func (w *Workspace) SolutionPath() string { return filepath.Join(w.Dir, solutionFile) }

// WriteSolution replaces the candidate source, e.g. with a mutant.
func (w *Workspace) WriteSolution(src string) error {
	return os.WriteFile(w.SolutionPath(), []byte(src), 0o644)
}

// cmdResult is the outcome of one child process.
type cmdResult struct {
	Output   string
	Duration time.Duration
	Err      error
	TimedOut bool
}

// run executes a go subcommand in the workspace. sandbox selects whether the
// exec wrapper is applied; it should be true for any stage that runs
// model-authored code.
func (w *Workspace) run(ctx context.Context, timeout time.Duration, sandbox bool, args ...string) cmdResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	argv := append([]string{w.r.goBin}, args...)
	if sandbox && len(w.r.execWrapper) > 0 {
		argv = append(append([]string{}, w.r.execWrapper...), argv...)
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = w.Dir
	cmd.Env = w.r.env
	// WaitDelay bounds the window between context cancellation and the child
	// actually dying, so a wedged test cannot pin a worker slot indefinitely.
	cmd.WaitDelay = 5 * time.Second

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	res := cmdResult{
		Output:   strings.TrimSpace(buf.String()),
		Duration: time.Since(start),
		Err:      err,
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		if res.Output == "" {
			res.Output = fmt.Sprintf("timed out after %s", timeout)
		}
	}
	return res
}
