package pkg

import "path/filepath"

// RuncRoot is the runc --root used for every kettle container.
const RuncRoot = "/run/kettle/containers"

// TTRPCSocketPath is where the shim for container id listens.
func TTRPCSocketPath(id string) string {
	return filepath.Join(RuncRoot, id, id+".ttrpc.sock")
}
