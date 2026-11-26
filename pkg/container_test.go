package pkg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestWriteDefaultSpec(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteDefaultSpec(dir))

	spec, err := loadSpec(filepath.Join(dir, "config.json"))
	require.NoError(t, err)
	assert.False(t, spec.Process.Terminal, "kettle has no console socket")
	assert.Equal(t, "rootfs", spec.Root.Path)
	assert.NotEmpty(t, spec.Process.Args)
}

func TestLoadSpec(t *testing.T) {
	valid := specs.Spec{Process: &specs.Process{Cwd: "/", Args: []string{"sh"}}}

	for _, tc := range []struct {
		name    string
		content any // nil: don't write the file
		wantErr string
	}{
		{name: "valid", content: valid},
		{name: "missing file", wantErr: "not found"},
		{name: "null", content: json.RawMessage("null"), wantErr: "config cannot be null"},
		{name: "invalid json", content: json.RawMessage(`"{"`), wantErr: "cannot unmarshal"},
		{name: "no process", content: specs.Spec{}, wantErr: "process property must not be empty"},
		{
			name:    "relative cwd",
			content: specs.Spec{Process: &specs.Process{Cwd: "rel", Args: []string{"sh"}}},
			wantErr: "cwd property must be an absolute path",
		},
		{
			name:    "empty cwd",
			content: specs.Spec{Process: &specs.Process{Args: []string{"sh"}}},
			wantErr: "cwd property must be an absolute path",
		},
		{
			name:    "no args",
			content: specs.Spec{Process: &specs.Process{Cwd: "/"}},
			wantErr: "args must not be empty",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if tc.content != nil {
				data, err := json.Marshal(tc.content)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, data, 0o644))
			}
			spec, err := loadSpec(path)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, valid.Process.Args, spec.Process.Args)
		})
	}
}

func TestResolveBundlePaths(t *testing.T) {
	const bundle = "/bundle"
	spec := &specs.Spec{
		Root: &specs.Root{Path: "rootfs"},
		Mounts: []specs.Mount{
			{Destination: "/proc", Type: "proc", Source: "proc"},
			{Destination: "/a", Type: "bind", Source: "data"},
			{Destination: "/b", Source: "data", Options: []string{"rbind", "ro"}},
			{Destination: "/c", Type: "bind", Source: "/abs"},
		},
	}
	resolveBundlePaths(spec, bundle)

	assert.Equal(t, "/bundle/rootfs", spec.Root.Path)
	assert.Equal(t, "proc", spec.Mounts[0].Source, "non-bind sources are not paths")
	assert.Equal(t, "/bundle/data", spec.Mounts[1].Source)
	assert.Equal(t, "/bundle/data", spec.Mounts[2].Source)
	assert.Equal(t, "/abs", spec.Mounts[3].Source)

	abs := &specs.Spec{Root: &specs.Root{Path: "/elsewhere"}}
	resolveBundlePaths(abs, bundle)
	assert.Equal(t, "/elsewhere", abs.Root.Path)

	// No root at all must not panic.
	resolveBundlePaths(&specs.Spec{}, bundle)
}

func TestIsBindMount(t *testing.T) {
	assert.True(t, isBindMount(specs.Mount{Type: "bind"}))
	assert.True(t, isBindMount(specs.Mount{Options: []string{"ro", "bind"}}))
	assert.True(t, isBindMount(specs.Mount{Options: []string{"rbind"}}))
	assert.False(t, isBindMount(specs.Mount{Type: "tmpfs", Options: []string{"nosuid"}}))
}

func TestSetBundleLabel(t *testing.T) {
	c := &configs.Config{Labels: []string{"foo=bar", "bundle=/old"}}
	setBundleLabel(c, "/new")
	assert.Equal(t, []string{"foo=bar", "bundle=/new"}, c.Labels)

	c = &configs.Config{Labels: []string{"foo=bar"}}
	setBundleLabel(c, "/new")
	assert.Equal(t, []string{"foo=bar", "bundle=/new"}, c.Labels)
}

func TestNewProcess(t *testing.T) {
	p := &specs.Process{
		Args: []string{"sh"},
		Env:  []string{"PATH=/bin"},
		Cwd:  "/",
		User: specs.User{UID: 1, GID: 2, AdditionalGids: []uint32{3, 4}},
		Capabilities: &specs.LinuxCapabilities{
			Bounding: []string{"CAP_KILL"},
		},
		Rlimits: []specs.POSIXRlimit{{Type: "RLIMIT_NOFILE", Hard: 1024, Soft: 512}},
	}
	lp, err := newProcess(p)
	require.NoError(t, err)

	assert.Equal(t, p.Args, lp.Args)
	assert.Equal(t, p.Env, lp.Env)
	assert.Equal(t, 1, lp.UID)
	assert.Equal(t, 2, lp.GID)
	assert.Equal(t, []int{3, 4}, lp.AdditionalGroups)
	assert.True(t, lp.Init)
	require.NotNil(t, lp.Capabilities)
	assert.Equal(t, []string{"CAP_KILL"}, lp.Capabilities.Bounding)
	assert.Equal(t, []configs.Rlimit{{Type: unix.RLIMIT_NOFILE, Hard: 1024, Soft: 512}}, lp.Rlimits)
	// runc create detaches the container onto the caller's stdio.
	assert.Equal(t, os.Stdin, lp.Stdin)
	assert.Equal(t, os.Stdout, lp.Stdout)
	assert.Equal(t, os.Stderr, lp.Stderr)
}

func TestNewProcessInvalidRlimit(t *testing.T) {
	_, err := newProcess(&specs.Process{
		Args:    []string{"sh"},
		Cwd:     "/",
		Rlimits: []specs.POSIXRlimit{{Type: "RLIMIT_BOGUS"}},
	})
	assert.ErrorContains(t, err, "wrong rlimit value")
}
