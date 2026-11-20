//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package prover

import "context"

const fileLockSupported = false

// lockFileExclusive is a no-op where advisory flock is unavailable, so
// concurrent calls are not serialized on those platforms.
func lockFileExclusive(ctx context.Context, path string) (func(), error) {
	return func() {}, nil
}
