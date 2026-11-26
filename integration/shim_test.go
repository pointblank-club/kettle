package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	task "kettle/api/shim"
	"kettle/pkg"
	"kettle/pkg/testutil"

	"github.com/containerd/ttrpc"
	"github.com/opencontainers/runc/libcontainer"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const waitTimeout = 10 * time.Second

func TestShimLayout(t *testing.T) {
	testutil.RequiresRoot(t)
	id := newID(t)
	startShim(t, id)

	// Mirrors containerd: <task dir>/address points at a hashed socket in s/.
	address, err := os.ReadFile(pkg.ShimAddressFile(id))
	require.NoError(t, err)
	assert.Equal(t, "unix://"+pkg.TTRPCSocketPath(id), string(address))

	fi, err := os.Stat(pkg.TTRPCSocketPath(id))
	require.NoError(t, err)
	assert.Equal(t, os.ModeSocket, fi.Mode().Type())

	// shim.pid names the running shim server, not the exited launcher.
	pid, err := pkg.ReadPidFile(pkg.ShimPidFile(id))
	require.NoError(t, err)
	cmdline, err := os.ReadFile(filepath.Join("/proc", itoa(pid), "cmdline"))
	require.NoError(t, err)
	args := strings.Split(strings.TrimRight(string(cmdline), "\x00"), "\x00")
	require.Len(t, args, 3)
	assert.Equal(t, "kettle-shim", filepath.Base(args[0]))
	assert.Equal(t, []string{"--id", id}, args[1:])
}

func TestShimStartTwice(t *testing.T) {
	testutil.RequiresRoot(t)
	id := newID(t)
	startShim(t, id)

	out, err := exec.Command("kettle-shim", "start", "--id", id).CombinedOutput()
	assert.Error(t, err)
	assert.Contains(t, string(out), "already running")
}

func TestShimStartInvalidID(t *testing.T) {
	testutil.RequiresRoot(t)
	out, err := exec.Command("kettle-shim", "start", "--id", "../escape").CombinedOutput()
	assert.Error(t, err)
	assert.Contains(t, string(out), "invalid container id")
}

func TestContainerLifecycle(t *testing.T) {
	testutil.RequiresRoot(t)
	ctx := context.Background()
	id := newID(t)
	bundle := newBundle(t, "/bin/busybox", "true")
	shim := startShim(t, id)

	created, err := shim.Create(ctx, &task.CreateTaskRequest{Id: id, Bundle: bundle})
	require.NoError(t, err)
	assert.NotZero(t, created.Pid)
	assert.Equal(t, libcontainer.Created, containerStatus(t, id))

	started, err := shim.Start(ctx, &task.StartRequest{ContainerId: id})
	require.NoError(t, err)
	assert.Equal(t, created.Pid, started.Pid, "start execs the init created by create")

	require.Eventually(t, func() bool {
		return containerStatus(t, id) == libcontainer.Stopped
	}, waitTimeout, 50*time.Millisecond, "container should exit")

	_, err = shim.Delete(ctx, &task.DeleteRequest{Id: id})
	require.NoError(t, err)
	assertCleanedUp(t, id)
}

func TestStartTwice(t *testing.T) {
	testutil.RequiresRoot(t)
	ctx := context.Background()
	id := newID(t)
	bundle := newBundle(t, "/bin/busybox", "sleep", "30")
	shim := startShim(t, id)

	_, err := shim.Create(ctx, &task.CreateTaskRequest{Id: id, Bundle: bundle})
	require.NoError(t, err)
	_, err = shim.Start(ctx, &task.StartRequest{ContainerId: id})
	require.NoError(t, err)

	_, err = shim.Start(ctx, &task.StartRequest{ContainerId: id})
	assert.ErrorContains(t, err, "already running")

	_, err = shim.Delete(ctx, &task.DeleteRequest{Id: id})
	assert.ErrorContains(t, err, "not stopped", "like runc delete without --force")

	// There is no Kill yet; stop it behind the shim's back.
	c, err := libcontainer.Load(pkg.RuncRoot, id)
	require.NoError(t, err)
	require.NoError(t, c.Signal(os.Kill))
	require.Eventually(t, func() bool {
		return containerStatus(t, id) == libcontainer.Stopped
	}, waitTimeout, 50*time.Millisecond)

	_, err = shim.Delete(ctx, &task.DeleteRequest{Id: id})
	require.NoError(t, err)
	assertCleanedUp(t, id)
}

// Deleting a created (never started) container kills its init, as runc does.
func TestDeleteCreated(t *testing.T) {
	testutil.RequiresRoot(t)
	ctx := context.Background()
	id := newID(t)
	bundle := newBundle(t, "/bin/busybox", "true")
	shim := startShim(t, id)

	_, err := shim.Create(ctx, &task.CreateTaskRequest{Id: id, Bundle: bundle})
	require.NoError(t, err)

	_, err = shim.Delete(ctx, &task.DeleteRequest{Id: id})
	require.NoError(t, err)
	assertCleanedUp(t, id)
}

// A shim exists for one container only, so a failed Create shuts it down.
func TestCreateFailureStopsShim(t *testing.T) {
	testutil.RequiresRoot(t)
	ctx := context.Background()
	id := newID(t)
	shim := startShim(t, id)

	_, err := shim.Create(ctx, &task.CreateTaskRequest{Id: id, Bundle: t.TempDir()}) // no config.json
	assert.ErrorContains(t, err, "not found")
	assertCleanedUp(t, id)
}

// The container's init is reparented to the shim (a child subreaper), not to
// PID 1, and the shim is not a child of whoever launched it.
func TestShimIsContainerParent(t *testing.T) {
	testutil.RequiresRoot(t)
	ctx := context.Background()
	id := newID(t)
	bundle := newBundle(t, "/bin/busybox", "sleep", "30")
	shim := startShim(t, id)

	created, err := shim.Create(ctx, &task.CreateTaskRequest{Id: id, Bundle: bundle})
	require.NoError(t, err)

	shimPid := ppid(t, int(created.Pid))
	exe, err := os.Readlink(filepath.Join("/proc", itoa(shimPid), "exe"))
	require.NoError(t, err)
	assert.Equal(t, "kettle-shim", filepath.Base(exe))
	recorded, err := pkg.ReadPidFile(pkg.ShimPidFile(id))
	require.NoError(t, err)
	assert.Equal(t, recorded, shimPid, "shim.pid must name the container's parent")
	assert.NotEqual(t, os.Getpid(), ppid(t, shimPid), "the shim must be detached from its launcher")

	_, err = shim.Delete(ctx, &task.DeleteRequest{Id: id})
	require.NoError(t, err)
	assertCleanedUp(t, id)
}

// newID returns a fresh container id and makes sure nothing is left behind
// if the test fails half way.
func newID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	id := "kettle-it-" + hex.EncodeToString(b)
	t.Cleanup(func() { forceCleanup(id) })
	return id
}

// startShim launches a shim the way the daemon does and connects to it.
func startShim(t *testing.T, id string) task.TaskService {
	t.Helper()
	cmd := exec.Command("kettle-shim", "start", "--id", id)
	// Must be *os.File: the shim inherits them and outlives the launcher.
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	require.NoError(t, cmd.Run())

	shim, err := dialShim(id)
	require.NoError(t, err)
	return shim
}

// dialShim connects to the shim for id like the daemon's dialShim does. The
// connection is closed when the shim shuts down.
func dialShim(id string) (task.TaskService, error) {
	conn, err := net.Dial("unix", pkg.TTRPCSocketPath(id))
	if err != nil {
		return nil, err
	}
	return task.NewTaskClient(ttrpc.NewClient(conn)), nil
}

func containerStatus(t *testing.T, id string) libcontainer.Status {
	t.Helper()
	c, err := libcontainer.Load(pkg.RuncRoot, id)
	require.NoError(t, err)
	s, err := c.Status()
	require.NoError(t, err)
	return s
}

// assertCleanedUp checks that the shim exited and removed its state, and
// that no container state is left.
func assertCleanedUp(t *testing.T, id string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := net.Dial("unix", pkg.TTRPCSocketPath(id))
		return err != nil
	}, waitTimeout, 50*time.Millisecond, "shim should stop serving")
	require.Eventually(t, func() bool {
		return !exists(pkg.TTRPCSocketPath(id)) && !exists(pkg.ShimDir(id))
	}, waitTimeout, 50*time.Millisecond, "shim should remove its socket and directory")
	assert.NoDirExists(t, filepath.Join(pkg.RuncRoot, id))
}

func forceCleanup(id string) {
	if c, err := libcontainer.Load(pkg.RuncRoot, id); err == nil {
		_ = c.Signal(os.Kill)
		time.Sleep(200 * time.Millisecond)
		_ = c.Destroy()
	}
	if shim, err := dialShim(id); err == nil {
		_, _ = shim.Delete(context.Background(), &task.DeleteRequest{Id: id})
	}
	killShim(id)
	_ = os.RemoveAll(filepath.Join(pkg.RuncRoot, id))
	_ = os.RemoveAll(pkg.ShimDir(id))
	_ = os.Remove(pkg.TTRPCSocketPath(id))
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return !errors.Is(err, os.ErrNotExist)
}

func setArgs(t *testing.T, bundle string, args []string) {
	t.Helper()
	path := filepath.Join(bundle, "config.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var spec specs.Spec
	require.NoError(t, json.Unmarshal(data, &spec))
	spec.Process.Args = args
	data, err = json.Marshal(&spec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

// killShim kills the shim serving id, found through its shim.pid file.
func killShim(id string) {
	if pid, err := pkg.ReadPidFile(pkg.ShimPidFile(id)); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// ppid returns the parent pid of pid, from /proc/<pid>/stat.
func ppid(t *testing.T, pid int) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("/proc", itoa(pid), "stat"))
	require.NoError(t, err)
	// The command name (field 2) may contain spaces; fields after it don't.
	fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
	require.GreaterOrEqual(t, len(fields), 2)
	parent, err := strconv.Atoi(fields[1]) // field 4: state is fields[0]
	require.NoError(t, err)
	return parent
}

func itoa(i int) string { return strconv.Itoa(i) }
