package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExistingVMStartDoesNotReplaceConfiguration(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	cli := filepath.Join(bin, "limactl")
	script := "#!/bin/sh\nif [ \"$1\" = list ]; then printf 'ci-self-x64\\n'; exit 0; fi\nprintf '%s\\n' \"$@\" > \"$VM_TEST_LOG\"\n"
	if err := os.WriteFile(cli, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "qemu-system-x86_64"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CI_SELF_LIMACTL", cli)
	t.Setenv("VM_TEST_LOG", log)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if err := run(context.Background(), []string{"start", "--config", "/nonexistent/should-not-be-used.yaml"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "start\nci-self-x64\n") || strings.Contains(string(b), "yaml") {
		t.Fatalf("existing configuration replaced: %s", b)
	}
}

func TestVMCommandsRejectUnrelatedNamesAndDeletion(t *testing.T) {
	for _, args := range [][]string{{"stop", "--name", "unrelated"}, {"stop", "--name", "../ci-self-x64"}, {"delete"}} {
		if err := run(context.Background(), args); err == nil {
			t.Fatalf("accepted unsupported operation: %v", args)
		}
	}
}
