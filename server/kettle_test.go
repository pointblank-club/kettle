package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBundle(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "bundle")
	require.NoError(t, createBundle(bundle))

	fi, err := os.Stat(filepath.Join(bundle, "rootfs"))
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
	_, err = os.Stat(filepath.Join(bundle, "config.json"))
	assert.NoError(t, err, "a default spec is generated")
}

func TestCreateBundleKeepsExistingSpec(t *testing.T) {
	bundle := t.TempDir()
	config := filepath.Join(bundle, "config.json")
	require.NoError(t, os.WriteFile(config, []byte(`{"custom":true}`), 0o644))

	require.NoError(t, createBundle(bundle))

	data, err := os.ReadFile(config)
	require.NoError(t, err)
	assert.JSONEq(t, `{"custom":true}`, string(data))
}
