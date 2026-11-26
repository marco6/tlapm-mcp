package prover

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// moduleCacheKey returns the key TLAPM uses for a module's cache directory
// (.tlacache/<key>.tlaps). TLAPM keys that directory by the module's declared
// name, which by convention matches the file name without its .tla extension.
// The MCP does not read module contents, so the file base name is the proxy;
// aliases such as "Spec.tla" and "dir/Spec.tla" deliberately share a lock.
func moduleCacheKey(module string) string {
	base := filepath.Base(filepath.Clean(module))
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".tla") {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" || base == "." || base == string(filepath.Separator) {
		return module
	}
	return base
}

// moduleLockPath returns the sidecar file used for module locking.
// It follows TLAPM_CACHE_DIR so the lock lives beside the cache it protects.
// The file is never removed: unlinking a path while another process holds or
// waits on its inode lets a third process create a new inode and bypass it.
func moduleLockPath(module string) (string, error) {
	cacheDir := os.Getenv("TLAPM_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = ".tlacache"
	}
	if abs, err := filepath.Abs(cacheDir); err == nil {
		cacheDir = abs
	}
	lockDir := filepath.Join(cacheDir, ".tlapm-mcp-locks")
	if err := os.MkdirAll(lockDir, 0o777); err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(lockDir); err == nil {
		lockDir = resolved
	}
	sum := sha256.Sum256([]byte(moduleCacheKey(module)))
	return filepath.Join(lockDir, hex.EncodeToString(sum[:])+".lock"), nil
}

// acquireModuleLock blocks until the module's advisory sidecar lock is held.
// It returns the context error if ctx is cancelled while waiting, or a
// filesystem error if the sidecar cannot be locked. On platforms without
// advisory flock the returned release function is a no-op.
func acquireModuleLock(ctx context.Context, module string) (func(), error) {
	if !fileLockSupported {
		return func() {}, nil
	}
	path, err := moduleLockPath(module)
	if err != nil {
		return nil, err
	}
	return lockFileExclusive(ctx, path)
}
