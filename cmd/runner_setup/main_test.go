package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLabelsForMobileProfileAll(t *testing.T) {
	got, err := labelsForMobileProfile("self-hosted,mac-mini,fastlane", "all")
	if err != nil {
		t.Fatalf("labelsForMobileProfile returned error: %v", err)
	}

	want := "self-hosted,mac-mini,fastlane,mobile,ios,android,xcode,android-sdk"
	if got != want {
		t.Fatalf("unexpected labels\nwant: %s\n got: %s", want, got)
	}
}

func TestLabelsForMobileProfileInvalid(t *testing.T) {
	_, err := labelsForMobileProfile(defaultLabels, "windows-phone")
	if err == nil {
		t.Fatal("expected invalid mobile profile to fail")
	}
	if !strings.Contains(err.Error(), "invalid_mobile_profile=windows-phone") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunnerHashesConfigured(t *testing.T) {
	for _, arch := range []string{"osx-x64", "osx-arm64"} {
		hash := runnerHashes[arch]
		if len(hash) != 64 {
			t.Fatalf("runner hash for %s should be 64 hex chars, got %q", arch, hash)
		}
	}
}

func TestVerifySHA256RejectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actions-runner-osx-arm64-"+runnerVersion+".tar.gz")
	if err := os.WriteFile(path, []byte("not the runner tarball"), 0o644); err != nil {
		t.Fatalf("write test tarball failed: %v", err)
	}

	err := verifySHA256("osx-arm64", path)
	if err == nil {
		t.Fatal("expected sha256 mismatch")
	}
	if !strings.Contains(err.Error(), "sha256_mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}
