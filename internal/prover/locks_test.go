package prover

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestModuleCacheKey(t *testing.T) {
	tests := []struct {
		module string
		want   string
	}{
		{module: "Spec.tla", want: "Spec"},
		{module: "./Spec.tla", want: "Spec"},
		{module: "dir/Spec.tla", want: "Spec"},
		{module: "/abs/dir/Spec.tla", want: "Spec"},
		{module: "Spec", want: "Spec"},
		{module: "Spec.TLA", want: "Spec"},
		{module: "dir/Spec.tla.bak", want: "Spec.tla.bak"},
		{module: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			if got := moduleCacheKey(tt.module); got != tt.want {
				t.Fatalf("moduleCacheKey(%q) = %q, want %q", tt.module, got, tt.want)
			}
		})
	}
}

func TestModuleLockPath(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("TLAPM_CACHE_DIR", cacheDir)

	first, err := moduleLockPath("one/Shared.tla")
	if err != nil {
		t.Fatalf("moduleLockPath() error = %v", err)
	}
	second, err := moduleLockPath("two/Shared.tla")
	if err != nil {
		t.Fatalf("moduleLockPath() error = %v", err)
	}
	if first != second {
		t.Fatalf("path aliases produced different lock files: %q != %q", first, second)
	}
	other, err := moduleLockPath("Other.tla")
	if err != nil {
		t.Fatalf("moduleLockPath() error = %v", err)
	}
	if first == other {
		t.Fatalf("different modules share lock file %q", first)
	}

	resolved, err := filepath.EvalSymlinks(cacheDir)
	if err != nil {
		resolved = cacheDir
	}
	wantDir := filepath.Join(resolved, ".tlapm-mcp-locks")
	if filepath.Dir(first) != wantDir {
		t.Fatalf("lock dir = %q, want %q", filepath.Dir(first), wantDir)
	}
}

func TestProveSerializesSameModule(t *testing.T) {
	fix := newConcurrencyFixture(t)
	module := writeTestModule(t, fix.root, "Same.tla")
	p := New()
	defer fix.release()

	outcomes := make(chan proveOutcome, 2)
	go func() {
		result, err := p.Prove(context.Background(), ProveArgs{
			Module: module,
			Target: LineTarget{Line: 1},
		})
		outcomes <- proveOutcome{result: result, err: err}
	}()

	waitForCondition(t, 5*time.Second, "first same-module prove to start", func() bool {
		return fix.count("start Same ") == 1
	})

	go func() {
		result, err := p.Prove(context.Background(), ProveArgs{
			Module: module,
			Target: LineTarget{Line: 1},
		})
		outcomes <- proveOutcome{result: result, err: err}
	}()

	// The first helper is deliberately held open. A second helper must not
	// enter, or it will record an overlap in the event log.
	time.Sleep(500 * time.Millisecond)
	if got := fix.count("start Same "); got != 1 {
		t.Fatalf("same-module prove calls overlapped: %d starts", got)
	}

	fix.release()
	first := waitForProve(t, outcomes)
	second := waitForProve(t, outcomes)
	for i, out := range []proveOutcome{first, second} {
		if out.err != nil {
			t.Fatalf("prove call %d error = %v", i+1, out.err)
		}
		if !out.result.Success {
			t.Fatalf("prove call %d result = %+v, want success", i+1, out.result)
		}
	}
	if got := fix.count("overlap Same"); got != 0 {
		t.Fatalf("same-module prove calls overlapped: %d overlap events", got)
	}
}

func TestProveRunsDifferentModulesConcurrently(t *testing.T) {
	fix := newConcurrencyFixture(t)
	alpha := writeTestModule(t, fix.root, "Alpha.tla")
	beta := writeTestModule(t, fix.root, "Beta.tla")
	p := New()
	defer fix.release()

	outcomes := make(chan proveOutcome, 2)
	start := func(module string) {
		result, err := p.Prove(context.Background(), ProveArgs{
			Module: module,
			Target: LineTarget{Line: 1},
		})
		outcomes <- proveOutcome{result: result, err: err}
	}
	go start(alpha)
	go start(beta)

	waitForCondition(t, 5*time.Second, "both different-module proves to start", func() bool {
		return fix.count("start Alpha ") == 1 && fix.count("start Beta ") == 1
	})

	fix.release()
	first := waitForProve(t, outcomes)
	second := waitForProve(t, outcomes)
	for i, out := range []proveOutcome{first, second} {
		if out.err != nil {
			t.Fatalf("prove call %d error = %v", i+1, out.err)
		}
		if !out.result.Success {
			t.Fatalf("prove call %d result = %+v, want success", i+1, out.result)
		}
	}
}

func TestCheckAndProveShareModuleLock(t *testing.T) {
	fix := newConcurrencyFixture(t)
	module := writeTestModule(t, fix.root, "Cross.tla")
	p := New()
	defer fix.release()

	checks := make(chan checkOutcome, 1)
	proves := make(chan proveOutcome, 1)
	go func() {
		result, err := p.Check(context.Background(), CheckArgs{Module: module})
		checks <- checkOutcome{result: result, err: err}
	}()

	waitForCondition(t, 5*time.Second, "check to start", func() bool {
		return fix.count("start Cross ") == 1
	})

	go func() {
		result, err := p.Prove(context.Background(), ProveArgs{
			Module: module,
			Target: LineTarget{Line: 1},
		})
		proves <- proveOutcome{result: result, err: err}
	}()

	time.Sleep(500 * time.Millisecond)
	if got := fix.count("start Cross "); got != 1 {
		t.Fatalf("check and prove overlapped: %d starts", got)
	}

	fix.release()
	check := waitForCheck(t, checks)
	prove := waitForProve(t, proves)
	if check.err != nil {
		t.Fatalf("check error = %v", check.err)
	}
	if !check.result.Success {
		t.Fatalf("check result = %+v, want success", check.result)
	}
	if prove.err != nil {
		t.Fatalf("prove error = %v", prove.err)
	}
	if !prove.result.Success {
		t.Fatalf("prove result = %+v, want success", prove.result)
	}
}

func TestModuleLockWaitHonorsContext(t *testing.T) {
	fix := newConcurrencyFixture(t)
	module := writeTestModule(t, fix.root, "Gate.tla")
	p := New()
	defer fix.release()

	first := make(chan proveOutcome, 1)
	go func() {
		result, err := p.Prove(context.Background(), ProveArgs{
			Module: module,
			Target: LineTarget{Line: 1},
		})
		first <- proveOutcome{result: result, err: err}
	}()

	waitForCondition(t, 5*time.Second, "first prove to start", func() bool {
		return fix.count("start Gate ") == 1
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := p.Prove(ctx, ProveArgs{
		Module: module,
		Target: LineTarget{Line: 1},
	})
	var toolErr *TLAPMError
	if !errors.As(err, &toolErr) || toolErr.Code != ErrRequestTimeout {
		t.Fatalf("queued prove error = %v, want code %q", err, ErrRequestTimeout)
	}
	if got := fix.count("start Gate "); got != 1 {
		t.Fatalf("cancelled prove started a helper: %d starts", got)
	}

	fix.release()
	if out := waitForProve(t, first); out.err != nil || !out.result.Success {
		t.Fatalf("first prove = %+v, err = %v, want success", out.result, out.err)
	}
}

func TestTLAPMHelperProcess(t *testing.T) {
	if os.Getenv("TLAPM_HELPER") != "1" {
		t.Skip("helper process only")
	}
	os.Exit(runTLAPMHelper())
}

type concurrencyFixture struct {
	root    string
	syncDir string
}

func newConcurrencyFixture(t *testing.T) concurrencyFixture {
	t.Helper()
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	syncDir := filepath.Join(root, "sync")
	for _, dir := range []string{binDir, syncDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}

	script := "#!/bin/sh\nexec \"$TLAPM_HELPER_BIN\" -test.run=^TestTLAPMHelperProcess$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "tlapm"), []byte(script), 0o700); err != nil {
		t.Fatalf("write fake tlapm: %v", err)
	}
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("TLAPM_HELPER", "1")
	t.Setenv("TLAPM_HELPER_BIN", self)
	t.Setenv("TLAPM_HELPER_DIR", syncDir)
	t.Setenv("TLAPM_CACHE_DIR", filepath.Join(root, "cache"))
	return concurrencyFixture{root: root, syncDir: syncDir}
}

func (f concurrencyFixture) release() {
	_ = os.WriteFile(filepath.Join(f.syncDir, "release"), []byte("go"), 0o600)
}

func (f concurrencyFixture) count(prefix string) int {
	data, err := os.ReadFile(filepath.Join(f.syncDir, "events"))
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

func writeTestModule(t *testing.T, dir, name string) string {
	t.Helper()
	moduleName := strings.TrimSuffix(name, filepath.Ext(name))
	module := filepath.Join(dir, name)
	source := fmt.Sprintf("---- MODULE %s ----\n====\n", moduleName)
	if err := os.WriteFile(module, []byte(source), 0o600); err != nil {
		t.Fatalf("write module %s: %v", module, err)
	}
	return module
}

func runTLAPMHelper() int {
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "fake tlapm: missing module argument")
		return 2
	}
	module := args[len(args)-1]
	key := moduleCacheKey(module)
	syncDir := os.Getenv("TLAPM_HELPER_DIR")
	if syncDir == "" {
		fmt.Fprintln(os.Stderr, "fake tlapm: TLAPM_HELPER_DIR is empty")
		return 2
	}

	appendEvent := func(format string, args ...any) {
		file, err := os.OpenFile(filepath.Join(syncDir, "events"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(file, format+"\n", args...)
		_ = file.Close()
	}

	lockDir := filepath.Join(syncDir, key+".lock")
	if err := os.Mkdir(lockDir, 0o700); err != nil {
		appendEvent("overlap %s", key)
		return 42
	}
	defer os.Remove(lockDir)

	appendEvent("start %s %d", key, os.Getpid())
	release := filepath.Join(syncDir, "release")
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(release); err == nil {
			break
		}
		if time.Now().After(deadline) {
			appendEvent("timeout %s", key)
			return 43
		}
		time.Sleep(5 * time.Millisecond)
	}
	appendEvent("end %s %d", key, os.Getpid())
	fmt.Println("[INFO]: All 1 obligations proved.")
	return 0
}

type proveOutcome struct {
	result Result
	err    error
}

type checkOutcome struct {
	result CheckResult
	err    error
}

func waitForCondition(t *testing.T, timeout time.Duration, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForProve(t *testing.T, outcomes <-chan proveOutcome) proveOutcome {
	t.Helper()
	select {
	case outcome := <-outcomes:
		return outcome
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for prove result")
		return proveOutcome{}
	}
}

func waitForCheck(t *testing.T, outcomes <-chan checkOutcome) checkOutcome {
	t.Helper()
	select {
	case outcome := <-outcomes:
		return outcome
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for check result")
		return checkOutcome{}
	}
}
