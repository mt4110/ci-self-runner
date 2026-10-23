package main

import (
	"strings"
	"testing"
)

func TestNixCacheIdentityAndMounts(t *testing.T) {
	images := []image{{Nix: "nix@sha256:first"}}
	c := nixCacheFor("/project", "linux/amd64", images)
	if duplicated := nixCacheFor("/project", "linux/amd64", append(images, image{Label: "ubuntu-22.04", Nix: images[0].Nix})); duplicated.Key != c.Key {
		t.Fatal("runner label duplication fragmented an identical Nix runtime cache")
	}
	for _, other := range []nixCacheVolumes{
		nixCacheFor("/other", "linux/amd64", images),
		nixCacheFor("/project", "linux/arm64", images),
		nixCacheFor("/project", "linux/amd64", []image{{Nix: "nix@sha256:second"}}),
	} {
		if other.Key == c.Key {
			t.Fatal("cache identity collision")
		}
	}
	args := strings.Join(actArgs(options{event: "push", runID: "run-test"}, executionPlan{NixCache: &c}, "/copy", "/control", "/artifacts"), " ")
	for _, want := range []string{"--volume=" + c.Store + ":/nix", "--volume=" + c.Evaluation + ":/root/.cache/nix", "--label=ci-self.run=run-test", "--container-daemon-socket -"} {
		if !strings.Contains(args, want) {
			t.Fatalf("missing %s: %s", want, args)
		}
	}
}

func TestNixCacheLockUsesStableEngineIdentityAndReleases(t *testing.T) {
	// Isolate platform-specific user cache paths without touching production locks.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", home)
	unlock, err := lockNixCache("key", "engine-one")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if other, err := lockNixCache("key", "engine-one"); err == nil {
		other()
		t.Fatal("concurrent cache writer accepted")
	}
	unlock()
	again, err := lockNixCache("key", "engine-one")
	if err != nil {
		t.Fatal(err)
	}
	again()
}
