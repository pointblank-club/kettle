package pkg

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

// RuncRoot is the libcontainer state root used for every kettle container,
// like containerd's /run/containerd/runc.
const RuncRoot = "/run/kettle/runc"

// ShimRoot holds one directory per container shim. It must stay outside
// RuncRoot: libcontainer refuses to create a container whose state directory
// already exists, and the shim is running before the container is created.
const ShimRoot = "/run/kettle/shims"

// SocketRoot holds the shims' ttrpc sockets, like containerd's
// /run/containerd/s. Sockets are named by a hash rather than living in the
// shim's directory so their path length is fixed: unix socket paths are
// limited to ~108 bytes, and container ids have no length limit.
const SocketRoot = "/run/kettle/s"

// ShimDir is the per-container shim directory (containerd's
// io.containerd.runtime.v2.task/<ns>/<id>).
func ShimDir(id string) string {
	return filepath.Join(ShimRoot, id)
}

// ShimAddressFile records the shim's socket address, as containerd's
// <bundle>/address does.
func ShimAddressFile(id string) string {
	return filepath.Join(ShimDir(id), "address")
}

// TTRPCSocketPath is where the shim for container id listens.
func TTRPCSocketPath(id string) string {
	sum := sha256.Sum256([]byte(filepath.Join(ShimRoot, id)))
	return filepath.Join(SocketRoot, fmt.Sprintf("%x", sum))
}

// ValidateID rejects container ids that libcontainer would reject. Ids become
// path components under RuncRoot and ShimRoot, so this also keeps them from
// escaping those directories (no "/", ".", or "..").
func ValidateID(id string) error {
	if id == "" || id == "." || id == ".." {
		return fmt.Errorf("invalid container id %q", id)
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '_', c == '+', c == '-', c == '.':
		default:
			return fmt.Errorf("invalid container id %q: only [A-Za-z0-9_+.-] allowed", id)
		}
	}
	return nil
}
