// Package testutil holds helpers shared by kettle's tests, modeled on
// containerd's pkg/testutil.
package testutil

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var rootEnabled bool

func init() {
	flag.BoolVar(&rootEnabled, "test.root", false, "enable tests that require root")
}

// RootEnabled reports whether -test.root was passed. Call it after flag.Parse
// (i.e. inside a test, or in TestMain after flag.Parse).
func RootEnabled() bool {
	return rootEnabled
}

// RequiresRoot skips the test unless -test.root was passed, and fails it if
// -test.root was passed but the test is not running as root.
func RequiresRoot(t testing.TB) {
	t.Helper()
	if !rootEnabled {
		t.Skip("skipping test that requires root")
	}
	assert.Equal(t, 0, os.Getuid(), "This test must be run as root.")
}
