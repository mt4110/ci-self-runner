package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

type nixCacheVolumes struct {
	Key        string `json:"key"`
	Store      string `json:"store_volume"`
	Evaluation string `json:"evaluation_volume"`
}

// Include the runtime recipe to avoid obscuring a changed Nix runtime with an
// old volume. Canonical project paths isolate unrelated checkouts by default.
func nixCacheFor(project, arch string, images []image) nixCacheVolumes {
	refs := make([]string, 0, len(images))
	seen := make(map[string]bool)
	for _, im := range images {
		if !seen[im.Nix] {
			refs = append(refs, im.Nix)
			seen[im.Nix] = true
		}
	}
	sort.Strings(refs)
	sum := sha256.Sum256([]byte(project + "\x00" + arch + "\x00" + dockerfile + "\x00" + strings.Join(refs, "\x00")))
	key := hex.EncodeToString(sum[:16])
	return nixCacheVolumes{key, "ci-self-nix-" + key, "ci-self-nix-eval-" + key}
}

func lockNixCache(key, engineID string) (func(), error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cache, "ci-self-runner", "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// Engine identity is stable even when SSH-forwarded socket names change.
	sum := sha256.Sum256([]byte(engineID + "\x00" + key))
	f, err := os.OpenFile(filepath.Join(dir, hex.EncodeToString(sum[:])+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("Nix cache is in use by another controller; retry after that run finishes")
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
