//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package prover

import (
	"context"
	"os"
	"sync"
	"syscall"
	"time"
)

const fileLockSupported = true

// lockFileExclusive opens or creates path and takes an exclusive advisory
// lock. It polls a non-blocking flock so context cancellation is honored; a
// blocking flock would pin an OS thread and ignore MCP deadlines.
func lockFileExclusive(ctx context.Context, path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return nil, err
	}
	fd := int(file.Fd())
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				_ = file.Close()
				return nil, ctxErr
			}
			var once sync.Once
			return func() {
				once.Do(func() {
					_ = file.Close()
				})
			}, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN && err != syscall.EINTR {
			_ = file.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
