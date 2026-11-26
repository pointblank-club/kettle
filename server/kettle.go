package server

import (
	"context"
	"fmt"
	pkg "kettle/pkg"
	"os"
	"path/filepath"

	containerTask "kettle/api/kettle"
	shimTask "kettle/api/shim"
)

type ContainerTaskServiceImpl struct {
	containerTask.UnimplementedContainersServer
}

func (s *ContainerTaskServiceImpl) Create(ctx context.Context, req *containerTask.CreateContainerRequest) (*containerTask.CreateContainerResponse, error) {
	fmt.Println("function create called on grpc")
	err := createContainer(ctx, req.Container.Bundle, req.Container.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	return &containerTask.CreateContainerResponse{}, nil
}

func (s *ContainerTaskServiceImpl) Start(ctx context.Context, req *containerTask.StartRequest) (*containerTask.StartResponse, error) {
	fmt.Println("function start called on grpc")
	if err := pkg.ValidateID(req.ContainerId); err != nil {
		return nil, err
	}
	shim, closeShim, err := dialShim(req.ContainerId)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	defer closeShim()
	resp, err := shim.Start(ctx, &shimTask.StartRequest{ContainerId: req.ContainerId})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	return &containerTask.StartResponse{Pid: resp.Pid}, nil
}

// createContainer prepares the bundle, then has a new shim create the
// container so that the shim, not the daemon, is the container's parent.
func createContainer(ctx context.Context, bundlePath, containerID string) error {
	if err := pkg.ValidateID(containerID); err != nil {
		return err
	}
	// The shim may run with a different cwd than the client meant.
	bundlePath, err := filepath.Abs(bundlePath)
	if err != nil {
		return err
	}
	if err := createBundle(bundlePath); err != nil {
		return err
	}
	// rootfs must contain the init binary before the container is created
	if err := pkg.InstallBusyBox(filepath.Join(bundlePath, "rootfs")); err != nil {
		return fmt.Errorf("install busybox: %w", err)
	}

	if err := launchShim(containerID); err != nil {
		return err
	}
	shim, closeShim, err := dialShim(containerID)
	if err != nil {
		return err
	}
	defer closeShim()
	if _, err := shim.Create(ctx, &shimTask.CreateTaskRequest{Id: containerID, Bundle: bundlePath}); err != nil {
		return err
	}
	fmt.Println("Container created:", containerID)
	return nil
}

func createBundle(bundlePath string) error {
	if err := os.MkdirAll(filepath.Join(bundlePath, "rootfs"), 0755); err != nil {
		return fmt.Errorf("failed to create bundle rootfs directory: %w", err)
	}
	configPath := filepath.Join(bundlePath, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := pkg.WriteDefaultSpec(bundlePath); err != nil {
			return fmt.Errorf("failed to generate default spec: %w", err)
		}
		fmt.Println("Default runc spec created at:", configPath)
	}
	return nil
}
