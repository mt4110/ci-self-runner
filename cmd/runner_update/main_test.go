package main

import "testing"

func TestNormalizeSemver(t *testing.T) {
	tests := map[string]string{
		"v2.334.0":              "2.334.0",
		"act version 0.2.87":    "0.2.87",
		"2.334.0-preview.1":     "2.334.0",
		" 2.334.0 darwin/arm64": "2.334.0",
		"":                      "",
	}
	for input, want := range tests {
		if got := normalizeSemver(input); got != want {
			t.Fatalf("normalizeSemver(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSemverLT(t *testing.T) {
	if !semverLT("2.321.0", "2.334.0") {
		t.Fatal("expected older runner version to compare lower")
	}
	if semverLT("2.334.0", "2.334.0") {
		t.Fatal("equal versions should not compare lower")
	}
	if semverLT("2.335.0", "2.334.0") {
		t.Fatal("newer versions should not compare lower")
	}
}
