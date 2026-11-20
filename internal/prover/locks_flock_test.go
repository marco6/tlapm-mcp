//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package prover

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProveWaitsForCrossProcessFileLock(t *testing.T) {
	fix := newConcurrencyFixture(t)
	module := writeTestModule(t, fix.root, "CrossAgent.tla")

	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	holder := exec.Command(self, "-test.run=^TestFileLockHolderProcess$", "--", module)
	holder.Env = append(os.Environ(), "TLAPM_LOCK_HOLDER=1")
	if err := holder.Start(); err != nil {
		t.Fatalf("start lock holder: %v", err)
	}
	waited := false
	defer func() {
		if waited {
			return
		}
		fix.release()
		_ = os.WriteFile(filepath.Join(fix.syncDir, "lock-release"), nil, 0o600)
		_ = holder.Wait()
	}()

	waitForCondition(t, 5*time.Second, "cross-process lock holder", func() bool {
		_, err := os.Stat(filepath.Join(fix.syncDir, "held"))
		return err == nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err = New().Prove(ctx, ProveArgs{
		Module: module,
		Target: LineTarget{Line: 1},
	})
	var toolErr *TLAPMError
	if !errors.As(err, &toolErr) || toolErr.Code != ErrRequestTimeout {
		t.Fatalf("prove behind a cross-process lock error = %v, want code %q", err, ErrRequestTimeout)
	}
	if got := fix.count("start CrossAgent "); got != 0 {
		t.Fatalf("prove started tlapm while another process held the module lock")
	}

	if err := os.WriteFile(filepath.Join(fix.syncDir, "lock-release"), nil, 0o600); err != nil {
		t.Fatalf("release lock holder: %v", err)
	}
	if err := holder.Wait(); err != nil {
		t.Fatalf("lock holder: %v", err)
	}
	waited = true

	fix.release()
	result, err := New().Prove(context.Background(), ProveArgs{
		Module: module,
		Target: LineTarget{Line: 1},
	})
	if err != nil {
		t.Fatalf("prove after releasing cross-process lock error = %v", err)
	}
	if !result.Success {
		t.Fatalf("prove after releasing cross-process lock = %+v, want success", result)
	}
}

func TestProveReturnsIOErrorWhenLockPathCannotBeCreated(t *testing.T) {
	fix := newConcurrencyFixture(t)
	module := writeTestModule(t, fix.root, "Blocked.tla")
	blocker := filepath.Join(fix.root, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	t.Setenv("TLAPM_CACHE_DIR", filepath.Join(blocker, "cache"))

	_, err := New().Prove(context.Background(), ProveArgs{
		Module: module,
		Target: LineTarget{Line: 1},
	})
	var toolErr *TLAPMError
	if !errors.As(err, &toolErr) || toolErr.Code != ErrIO {
		t.Fatalf("prove with unusable lock path error = %v, want code %q", err, ErrIO)
	}
	if got := fix.count("start Blocked "); got != 0 {
		t.Fatalf("prove started tlapm despite lock setup failure")
	}
}

func TestFileLockHolderProcess(t *testing.T) {
	if os.Getenv("TLAPM_LOCK_HOLDER") != "1" {
		t.Skip("helper process only")
	}
	os.Exit(runFileLockHolder())
}

func runFileLockHolder() int {
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "lock holder: missing module argument")
		return 2
	}
	release, err := acquireModuleLock(context.Background(), args[len(args)-1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock holder: acquire: %v\n", err)
		return 3
	}
	defer release()

	syncDir := os.Getenv("TLAPM_HELPER_DIR")
	if syncDir == "" {
		fmt.Fprintln(os.Stderr, "lock holder: TLAPM_HELPER_DIR is empty")
		return 2
	}
	if err := os.WriteFile(filepath.Join(syncDir, "held"), []byte("held"), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "lock holder: signal held: %v\n", err)
		return 3
	}

	releasePath := filepath.Join(syncDir, "lock-release")
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(releasePath); err == nil {
			return 0
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "lock holder: timed out waiting for release")
			return 4
		}
		time.Sleep(5 * time.Millisecond)
	}
}
