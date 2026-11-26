// Package integration runs kettle end to end, like containerd's
// integration/client: real shims, real containers, real /run/kettle state.
// The tests need root and only run with -test.root:
//
//	sudo -E go test -v ./integration -test.root
//
// or `make integration`. They also need network access once, to fetch busybox.
package integration

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kettle/pkg"
	"kettle/pkg/testutil"
)

// busyboxCache is a rootfs holding bin/busybox, shared by every test so that
// busybox is downloaded once per run.
var busyboxCache string

func TestMain(m *testing.M) {
	flag.Parse()
	if !testutil.RootEnabled() {
		// Every test calls RequiresRoot and will skip.
		os.Exit(m.Run())
	}
	os.Exit(run(m))
}

func run(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "kettle-integration-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(tmp)

	// The daemon execs "kettle-shim" from PATH, as containerd does for
	// containerd-shim-*; build the one under test and put it first.
	binDir := filepath.Join(tmp, "bin")
	// -buildvcs=false: under sudo, git refuses to read a repo owned by
	// another user ("dubious ownership"), which fails VCS stamping.
	build := exec.Command("go", "build", "-buildvcs=false", "-o", filepath.Join(binDir, "kettle-shim"), "kettle/cmd/kettle-shim")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to build kettle-shim:", err)
		return 1
	}
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	busyboxCache = filepath.Join(tmp, "busybox")
	if err := pkg.InstallBusyBox(busyboxCache); err != nil {
		fmt.Fprintln(os.Stderr, "failed to fetch busybox:", err)
		return 1
	}
	return m.Run()
}

// newBundle creates an OCI bundle whose rootfs holds busybox and whose
// process runs args.
func newBundle(t *testing.T, args ...string) string {
	t.Helper()
	bundle := t.TempDir()
	rootfs := filepath.Join(bundle, "rootfs")
	if err := os.MkdirAll(filepath.Join(rootfs, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(filepath.Join(busyboxCache, "bin", "busybox"), filepath.Join(rootfs, "bin", "busybox")); err != nil {
		t.Fatal(err)
	}
	if err := pkg.InstallBusyBox(rootfs); err != nil {
		t.Fatal(err)
	}
	if err := pkg.WriteDefaultSpec(bundle); err != nil {
		t.Fatal(err)
	}
	setArgs(t, bundle, args)
	return bundle
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
