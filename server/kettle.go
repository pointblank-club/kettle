package server

import (
	"context"
	"fmt"
	pkg "kettle/pkg"
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
	if err := createBundle(bundlePath); err != nil {
		return err
	}
	// rootfs must contain the init binary before runc create
	if err := pkg.InstallBusyBox(filepath.Join(bundlePath, "rootfs")); err != nil {
		return fmt.Errorf("install busybox: %w", err)
	}
	cmdCreate := exec.Command("runc", "--root", pkg.RuncRoot, "create", "--bundle", bundlePath, containerID)
	cmdCreate.Stdout = os.Stdout
	cmdCreate.Stderr = os.Stderr
	if err := cmdCreate.Run(); err != nil {
		return fmt.Errorf("failed to create container: %w", err)
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
		cmd := exec.Command("runc", "spec")
		cmd.Dir = bundlePath // Set working directory to bundle path
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to generate default spec: %w", err)
		}
		fmt.Println("Default runc spec created at:", configPath)
	}
	return nil
}
