package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Registers the cgroup device controller. Without it, creating a
	// container with device rules (specconv always adds the defaults) fails
	// with ErrDevicesUnsupported. runc's main.go imports it the same way.
	_ "github.com/opencontainers/cgroups/devices"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

// Container lifecycle on top of libcontainer, mirroring the semantics of the
// equivalent runc CLI commands (runc spec / create / start / delete) with
// --root RuncRoot.
//
// Any binary that calls CreateContainer must handle the "init" re-exec in its
// main package, see cmd/kettle/main.go.

// WriteDefaultSpec writes the default OCI spec to <bundlePath>/config.json,
// equivalent to `runc spec` run inside the bundle directory, but with
// process.terminal defaulting to false.
func WriteDefaultSpec(bundlePath string) error {
	spec := specconv.Example()
	spec.Process.Terminal = false
	data, err := json.MarshalIndent(spec, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(bundlePath, "config.json"), data, 0o666)
}

// CreateContainer is equivalent to `runc --root RuncRoot create --bundle bundlePath id`.
// The container is left in the created state, waiting for StartContainer.
func CreateContainer(bundlePath, id string) (retErr error) {
	bundlePath, err := filepath.Abs(bundlePath)
	if err != nil {
		return err
	}
	spec, err := loadSpec(filepath.Join(bundlePath, "config.json"))
	if err != nil {
		return err
	}
	// kettle has no console socket, so containers always run without a tty.
	spec.Process.Terminal = false
	resolveBundlePaths(spec, bundlePath)

	rootless := os.Geteuid() != 0
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:      id,
		Spec:            spec,
		RootlessEUID:    rootless,
		RootlessCgroups: rootless,
	})
	if err != nil {
		return err
	}
	// specconv labels the bundle with the cwd (runc chdirs into the bundle);
	// the daemon doesn't, so set it explicitly.
	setBundleLabel(config, bundlePath)

	container, err := libcontainer.Create(RuncRoot, id, config)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			_ = container.Destroy()
		}
	}()

	process, err := newProcess(spec.Process)
	if err != nil {
		return err
	}
	if err := container.Start(process); err != nil {
		return err
	}
	// runc create detaches and its init gets reparented to PID 1. Here the
	// init is our child, so reap it when it exits to avoid a zombie.
	go func() { _, _ = process.Wait() }()
	return nil
}

// StartContainer is equivalent to `runc --root RuncRoot start id`.
func StartContainer(id string) error {
	container, err := libcontainer.Load(RuncRoot, id)
	if err != nil {
		return err
	}
	status, err := container.Status()
	if err != nil {
		return err
	}
	switch status {
	case libcontainer.Created:
		return container.Exec()
	case libcontainer.Stopped:
		return errors.New("cannot start a container that has stopped")
	case libcontainer.Running:
		return errors.New("cannot start an already running container")
	default:
		return fmt.Errorf("cannot start a container in the %s state", status)
	}
}

// DeleteContainer is equivalent to `runc --root RuncRoot delete id`.
func DeleteContainer(id string) error {
	container, err := libcontainer.Load(RuncRoot, id)
	if err != nil {
		if errors.Is(err, libcontainer.ErrNotExist) {
			// Clean up leftover state, as runc does.
			_ = os.RemoveAll(filepath.Join(RuncRoot, id))
		}
		return err
	}
	status, err := container.Status()
	if err != nil {
		return err
	}
	switch status {
	case libcontainer.Stopped:
		return container.Destroy()
	case libcontainer.Created:
		return killContainer(container)
	default:
		return fmt.Errorf("cannot delete container %s that is not stopped: %s", id, status)
	}
}

func killContainer(container *libcontainer.Container) error {
	_ = container.Signal(unix.SIGKILL)
	for range 100 {
		time.Sleep(100 * time.Millisecond)
		if err := container.Signal(unix.Signal(0)); err != nil {
			return container.Destroy()
		}
	}
	return errors.New("container init still running")
}

func loadSpec(path string) (*specs.Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSON specification file %s not found", path)
		}
		return nil, err
	}
	var spec *specs.Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	if spec == nil {
		return nil, errors.New("config cannot be null")
	}
	if spec.Process == nil {
		return nil, errors.New("process property must not be empty")
	}
	if spec.Process.Cwd == "" || !filepath.IsAbs(spec.Process.Cwd) {
		return nil, errors.New("cwd property must be an absolute path")
	}
	if len(spec.Process.Args) == 0 {
		return nil, errors.New("args must not be empty")
	}
	return spec, nil
}

// resolveBundlePaths makes the paths that runc resolves against the bundle
// directory absolute, so conversion doesn't depend on the process cwd.
func resolveBundlePaths(spec *specs.Spec, bundlePath string) {
	if spec.Root != nil && !filepath.IsAbs(spec.Root.Path) {
		spec.Root.Path = filepath.Join(bundlePath, spec.Root.Path)
	}
	for i, m := range spec.Mounts {
		if m.Source != "" && !filepath.IsAbs(m.Source) && isBindMount(m) {
			spec.Mounts[i].Source = filepath.Join(bundlePath, m.Source)
		}
	}
}

func isBindMount(m specs.Mount) bool {
	if m.Type == "bind" {
		return true
	}
	for _, o := range m.Options {
		if o == "bind" || o == "rbind" {
			return true
		}
	}
	return false
}

func setBundleLabel(config *configs.Config, bundlePath string) {
	for i, l := range config.Labels {
		if strings.HasPrefix(l, "bundle=") {
			config.Labels[i] = "bundle=" + bundlePath
			return
		}
	}
	config.Labels = append(config.Labels, "bundle="+bundlePath)
}

// newProcess mirrors runc's newProcess plus the detached-stdio setup of
// `runc create`: the container inherits the caller's stdio.
func newProcess(p *specs.Process) (*libcontainer.Process, error) {
	lp := &libcontainer.Process{
		Args:            p.Args,
		Env:             p.Env,
		UID:             int(p.User.UID),
		GID:             int(p.User.GID),
		Cwd:             p.Cwd,
		Label:           p.SelinuxLabel,
		NoNewPrivileges: &p.NoNewPrivileges,
		AppArmorProfile: p.ApparmorProfile,
		Scheduler:       p.Scheduler,
		IOPriority:      p.IOPriority,
		Init:            true,
		Stdin:           os.Stdin,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
	}
	if p.Capabilities != nil {
		lp.Capabilities = &configs.Capabilities{
			Bounding:    p.Capabilities.Bounding,
			Effective:   p.Capabilities.Effective,
			Inheritable: p.Capabilities.Inheritable,
			Permitted:   p.Capabilities.Permitted,
			Ambient:     p.Capabilities.Ambient,
		}
	}
	for _, g := range p.User.AdditionalGids {
		lp.AdditionalGroups = append(lp.AdditionalGroups, int(g))
	}
	for _, r := range p.Rlimits {
		t, ok := rlimitMap[r.Type]
		if !ok {
			return nil, fmt.Errorf("wrong rlimit value: %s", r.Type)
		}
		lp.Rlimits = append(lp.Rlimits, configs.Rlimit{Type: t, Hard: r.Hard, Soft: r.Soft})
	}
	aff, err := configs.ConvertCPUAffinity(p.ExecCPUAffinity)
	if err != nil {
		return nil, err
	}
	lp.CPUAffinity = aff
	return lp, nil
}

var rlimitMap = map[string]int{
	"RLIMIT_CPU":        unix.RLIMIT_CPU,
	"RLIMIT_FSIZE":      unix.RLIMIT_FSIZE,
	"RLIMIT_DATA":       unix.RLIMIT_DATA,
	"RLIMIT_STACK":      unix.RLIMIT_STACK,
	"RLIMIT_CORE":       unix.RLIMIT_CORE,
	"RLIMIT_RSS":        unix.RLIMIT_RSS,
	"RLIMIT_NPROC":      unix.RLIMIT_NPROC,
	"RLIMIT_NOFILE":     unix.RLIMIT_NOFILE,
	"RLIMIT_MEMLOCK":    unix.RLIMIT_MEMLOCK,
	"RLIMIT_AS":         unix.RLIMIT_AS,
	"RLIMIT_LOCKS":      unix.RLIMIT_LOCKS,
	"RLIMIT_SIGPENDING": unix.RLIMIT_SIGPENDING,
	"RLIMIT_MSGQUEUE":   unix.RLIMIT_MSGQUEUE,
	"RLIMIT_NICE":       unix.RLIMIT_NICE,
	"RLIMIT_RTPRIO":     unix.RLIMIT_RTPRIO,
	"RLIMIT_RTTIME":     unix.RLIMIT_RTTIME,
}
