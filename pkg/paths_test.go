package pkg

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateID(t *testing.T) {
	for _, tc := range []struct {
		id    string
		valid bool
	}{
		{id: "abc", valid: true},
		{id: "A-b_c+d.e", valid: true},
		{id: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", valid: true},
		{id: "", valid: false},
		{id: ".", valid: false},
		{id: "..", valid: false},
		{id: "a/b", valid: false},
		{id: "../escape", valid: false},
		{id: "has space", valid: false},
		{id: "colon:", valid: false},
		{id: "nul\x00", valid: false},
	} {
		t.Run(tc.id, func(t *testing.T) {
			err := ValidateID(tc.id)
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestTTRPCSocketPath(t *testing.T) {
	a := TTRPCSocketPath("a")
	assert.Equal(t, SocketRoot, filepath.Dir(a), "sockets live in SocketRoot, like containerd's /run/containerd/s")
	assert.Equal(t, a, TTRPCSocketPath("a"), "socket path must be deterministic so the daemon can find a running shim")
	assert.NotEqual(t, a, TTRPCSocketPath("b"))

	// The name is a hash, so its length does not depend on the id and stays
	// under the unix socket path limit (sizeof(sun_path) = 108).
	long := TTRPCSocketPath(strings.Repeat("x", 4096))
	assert.Len(t, long, len(a))
	assert.Less(t, len(long), 108)
}

func TestShimDir(t *testing.T) {
	assert.Equal(t, filepath.Join(ShimRoot, "abc"), ShimDir("abc"))
	assert.Equal(t, filepath.Join(ShimRoot, "abc", "address"), ShimAddressFile("abc"))
	assert.Equal(t, filepath.Join(ShimRoot, "abc", "shim.pid"), ShimPidFile("abc"))
}

func TestRootsDoNotOverlap(t *testing.T) {
	// libcontainer refuses to create a container whose state directory
	// already exists, and the shim's directories exist before the container.
	roots := []string{RuncRoot, ShimRoot, SocketRoot}
	for _, a := range roots {
		for _, b := range roots {
			if a == b {
				continue
			}
			rel, err := filepath.Rel(a, b)
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(rel, ".."), "%s must not be inside %s", b, a)
		}
	}
}
