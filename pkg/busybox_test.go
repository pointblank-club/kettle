package pkg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An existing busybox must be reused, not downloaded again, so this test
// needs no network.
func TestInstallBusyBoxExisting(t *testing.T) {
	rootfs := t.TempDir()
	bb := filepath.Join(rootfs, "bin", "busybox")
	require.NoError(t, os.MkdirAll(filepath.Dir(bb), 0o755))
	require.NoError(t, os.WriteFile(bb, []byte("fake"), 0o755))

	for range 2 { // idempotent
		require.NoError(t, InstallBusyBox(rootfs))

		data, err := os.ReadFile(bb)
		require.NoError(t, err)
		assert.Equal(t, "fake", string(data))

		target, err := os.Readlink(filepath.Join(rootfs, "bin", "sh"))
		require.NoError(t, err)
		assert.Equal(t, "busybox", target, "relative, so it resolves inside the container")
	}
}
