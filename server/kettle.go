package server

import (
	"context"
	"fmt"
	pkg "kettle/pkg"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	containerTask "kettle/api/kettle"
	shimTask "kettle/api/shim"
)

type ContainerTaskServiceImpl struct {
	containerTask.UnimplementedContainersServer
}

func (s *ContainerTaskServiceImpl) Create(ctx context.Context, req *containerTask.CreateContainerRequest) (*containerTask.CreateContainerResponse, error) {
	fmt.Println("function create called on grpc")
	err := createContainer(req.Container.Bundle, req.Container.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	return &containerTask.CreateContainerResponse{}, nil
}

func (s *ContainerTaskServiceImpl) Start(ctx context.Context, req *containerTask.StartRequest) (*containerTask.StartResponse, error) {
	fmt.Println("function start called on grpc")
	pid, err := runShim(req.ContainerId)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	startReq := shimTask.StartRequest{
		ContainerId: req.ContainerId,
	}
	TaskServiceImpl.Start(TaskServiceImpl{}, ctx, &startReq)
	return &containerTask.StartResponse{Pid: pid}, nil
}
func createContainer(bundlePath, containerID string) error {
	createBundle(bundlePath)
	cmdCreate := exec.Command("runc", "--root", "/run/kettle/containers", "create", "--bundle", bundlePath, containerID)
	cmdCreate.Stdout = os.Stdout
	cmdCreate.Stderr = os.Stderr
	if err := cmdCreate.Run(); err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	pkg.InstallBusyBox(bundlePath)
	fmt.Println("Container created:", containerID)
	return nil
}

func createBundle(bundlePath string) error {
	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return fmt.Errorf("failed to create bundle directory: %w", err)
	}
	cmd := exec.Command("runc", "spec")
	cmd.Dir = bundlePath // Set working directory to bundle path
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate default spec: %w", err)
	}
	if err := os.MkdirAll(bundlePath+"/rootfs", 0755); err != nil {
		return fmt.Errorf("failed to create bundle rootfs directory: %w", err)
	}
	fmt.Println("Default runc spec created at:", bundlePath+"/config.json")

	return nil
}

func sstartShim(ctx context.Context, rootDir, id, namespace string) error {
	log.Printf("Starting shim for container %s in namespace %s", id, namespace)

	// Create a directory for the container
	containerDir := filepath.Join(rootDir, namespace, id)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		return fmt.Errorf("failed to create container directory: %w", err)
	}

	// Shim socket path
	shimSocketPath := filepath.Join(containerDir, "shim.sock")

	// Prepare the shim command
	cmd := exec.CommandContext(ctx, "/run/kettle/kettle.sock.ttrpc",
		"--namespace", namespace,
		"--id", id,
		"--address", shimSocketPath,
	)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the shim process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start shim process: %w", err)
	}

	// Don't wait for the process as it should run in the background
	log.Printf("Shim process started with PID %d", cmd.Process.Pid)

	return nil
}
