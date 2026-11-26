package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	task "kettle/api/shim"
	pkg "kettle/pkg"
)

type TaskServiceImpl struct{}

// is run by containerd daemon to call the shim binary
func runShim(id string) (pid uint32, err error) {
	cmdDelete := exec.Command("kettle-shim", "start", "--id", id)
	cmdDelete.Stdout = os.Stdout
	cmdDelete.Stderr = os.Stderr
	if err := cmdDelete.Run(); err != nil {
		return 0, fmt.Errorf("failed to create container shim: %w", err)
	}
	return pid, nil
}

// used by kettle shim to initialize itself
func StartShim(id string) (pid uint32, err error) {
	CreateTTRPCServer(context.TODO(), pkg.TTRPCSocketPath(id))
	return pid, nil
}
func (s TaskServiceImpl) Start(ctx context.Context, req *task.StartRequest) (*task.StartResponse, error) {
	log.Printf("Received Start request for ID: %s\n", req.ContainerId)
	startContainer(req.ContainerId)

	// Call your actual function here

	return &task.StartResponse{}, nil
}

func (s TaskServiceImpl) Delete(ctx context.Context, req *task.DeleteRequest) (*task.DeleteResponse, error) {
	log.Printf("Received Delete request for ID: %s\n", req.Id)
	if err := pkg.DeleteContainer(req.Id); err != nil {
		return nil, fmt.Errorf("failed to delete container: %w", err)
	}

	return &task.DeleteResponse{}, nil
}

func startContainer(id string) {
	if err := pkg.StartContainer(id); err != nil {
		log.Printf("failed to start container %s: %v\n", id, err)
		return
	}
	fmt.Println("Started container:", id)
}
