package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	containerTask "kettle/api/kettle"
	task "kettle/api/shim"
	pkg "kettle/pkg"

	"github.com/containerd/ttrpc"
	"google.golang.org/grpc"
)

func CreateGRPCServer(ctx context.Context) error {
	socketPath := "/run/kettle/kettle.sock"
	if err := os.MkdirAll("/run/kettle", 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := os.RemoveAll(socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	defer listener.Close()

	if err := os.Chmod(socketPath, 0666); err != nil {
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	server := grpc.NewServer()

	// Create and register your service
	containerTask.RegisterContainersServer(server, &ContainerTaskServiceImpl{}) // Note: usually ends with "Server"

	fmt.Println("gRPC server started on", socketPath)

	go func() {
		<-ctx.Done()
		fmt.Println("Shutting down gRPC server...")
		server.GracefulStop()
	}()

	if err := server.Serve(listener); err != nil {
		return fmt.Errorf("server stopped with error: %w", err)
	}

	return nil
}

// CreateTTRPCServer serves the shim API for container id until the shim is
// shut down (after Delete, or a failed Create).
func CreateTTRPCServer(ctx context.Context, id string) error {
	socketPath := pkg.TTRPCSocketPath(id)
	shimDir := pkg.ShimDir(id)
	for _, dir := range []string{shimDir, filepath.Dir(socketPath)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	defer os.RemoveAll(shimDir)
	defer os.Remove(socketPath)
	if err := os.RemoveAll(socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}
	if err := os.WriteFile(pkg.ShimAddressFile(id), []byte("unix://"+socketPath), 0644); err != nil {
		return fmt.Errorf("failed to write shim address: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}

	server, err := ttrpc.NewServer()
	if err != nil {
		return fmt.Errorf("failed to create ttrpc server: %w", err)
	}
	svc := &TaskServiceImpl{
		// Shutdown waits for in-flight requests, so it can't run inside one.
		shutdown: func() { go server.Shutdown(context.Background()) },
	}
	task.RegisterTaskService(server, svc)
	fmt.Println(" ttrpc server started on", socketPath)

	if err := server.Serve(ctx, listener); err != nil && !errors.Is(err, ttrpc.ErrServerClosed) {
		return fmt.Errorf("ttrpc server stopped: %w", err)
	}
	return nil
}
