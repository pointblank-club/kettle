package pkg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shim.pid")
	require.NoError(t, WritePidFile(path, 1234))
	pid, err := ReadPidFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1234, pid)

	// Overwrites, and leaves no temp file behind.
	require.NoError(t, WritePidFile(path, 42))
	pid, err = ReadPidFile(path)
	require.NoError(t, err)
	assert.Equal(t, 42, pid)
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestReadPidFileInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shim.pid")
	require.NoError(t, os.WriteFile(path, []byte("nope"), 0o644))
	_, err := ReadPidFile(path)
	assert.Error(t, err)

	_, err = ReadPidFile(filepath.Join(t.TempDir(), "missing"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
