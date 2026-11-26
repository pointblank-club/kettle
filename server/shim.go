package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	task "kettle/api/shim"
	pkg "kettle/pkg"

	"github.com/containerd/ttrpc"
	"golang.org/x/sys/unix"
)

// One kettle-shim process runs per container and is the container's parent:
//
//	kettle daemon ──exec──▶ kettle-shim start   (launcher, exits once the shim serves)
//	                              │
//	                              └─setsid─▶ kettle-shim --id X   (the shim, a child subreaper)
//	                                               │
//	                                               └──▶ container init   (reparented to the shim)
//
// The daemon then drives the container through the shim's ttrpc API.

const shimStartTimeout = 10 * time.Second

// launchShim starts the shim for container id. Used by the kettle daemon. It
// returns once the shim is serving; the shim itself is not the daemon's child.
func launchShim(id string) error {
	cmd := exec.Command("kettle-shim", "start", "--id", id)
	// These must be *os.File, not pipes: the shim inherits them and outlives
	// the launcher, and Run would wait for a pipe to close, i.e. forever.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container shim: %w", err)
	}
	return nil
}

// dialShim connects to the shim for container id. Used by the kettle daemon.
// The caller must call the returned close function.
func dialShim(id string) (task.TaskService, func(), error) {
	conn, err := net.Dial("unix", pkg.TTRPCSocketPath(id))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to shim for %s: %w", id, err)
	}
	client := ttrpc.NewClient(conn)
	return task.NewTaskClient(client), func() { client.Close() }, nil
}

// StartShim is `kettle-shim start`. It launches `kettle-shim --id <id>` as a
// detached process, waits until it is serving, and returns. Exiting here
// orphans the shim, so it is no longer tied to the daemon's lifetime.
func StartShim(id string) error {
	if err := pkg.ValidateID(id); err != nil {
		return err
	}
	socket := pkg.TTRPCSocketPath(id)
	if conn, err := net.Dial("unix", socket); err == nil {
		conn.Close()
		return fmt.Errorf("a shim for container %s is already running", id)
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(self, "--id", id) // no subcommand: server mode
	// The shim's stdout/stderr are the daemon's, and the container inherits them.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// New session: signals to the daemon's process group don't reach the shim.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch shim: %w", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.After(shimStartTimeout)
	for {
		if conn, err := net.Dial("unix", socket); err == nil {
			conn.Close()
			return nil
		}
		select {
		case err := <-exited:
			return fmt.Errorf("shim exited before serving: %v", err)
		case <-deadline:
			_ = cmd.Process.Kill()
			return fmt.Errorf("timed out waiting for shim socket %s", socket)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// ServeShim is `kettle-shim --id <id>`: the long-running shim for container id.
func ServeShim(id string) error {
	if err := pkg.ValidateID(id); err != nil {
		return err
	}
	// The container's init is forked by `kettle-shim init` and then orphaned;
	// as a child subreaper the shim adopts it instead of PID 1.
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("failed to become child subreaper: %w", err)
	}
	return CreateTTRPCServer(context.Background(), id)
}

// TaskServiceImpl is the shim's ttrpc service for a single container.
type TaskServiceImpl struct {
	// shutdown stops the shim once the response in flight has been sent.
	shutdown func()
}

func (s *TaskServiceImpl) Create(ctx context.Context, req *task.CreateTaskRequest) (*task.CreateTaskResponse, error) {
	log.Printf("Received Create request for ID: %s\n", req.Id)
	pid, err := pkg.CreateContainer(req.Bundle, req.Id)
	if err != nil {
		// Nothing to own: this shim exists only for this container.
		s.shutdown()
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	fmt.Println("Container created:", req.Id)
	return &task.CreateTaskResponse{Pid: uint32(pid)}, nil
}

func (s *TaskServiceImpl) Start(ctx context.Context, req *task.StartRequest) (*task.StartResponse, error) {
	log.Printf("Received Start request for ID: %s\n", req.ContainerId)
	pid, err := pkg.StartContainer(req.ContainerId)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	fmt.Println("Started container:", req.ContainerId)
	return &task.StartResponse{Pid: uint32(pid)}, nil
}

func (s *TaskServiceImpl) Delete(ctx context.Context, req *task.DeleteRequest) (*task.DeleteResponse, error) {
	log.Printf("Received Delete request for ID: %s\n", req.Id)
	if err := pkg.DeleteContainer(req.Id); err != nil {
		return nil, fmt.Errorf("failed to delete container: %w", err)
	}
	// The container is gone, so the shim has nothing left to do.
	s.shutdown()
	return &task.DeleteResponse{Id: req.Id}, nil
}
