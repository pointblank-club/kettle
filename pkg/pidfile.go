package pkg

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WritePidFile atomically writes pid to path, like containerd's
// shim.WritePidFile: readers never see a partial file.
func WritePidFile(path string, pid int) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path))
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ReadPidFile reads a pid written by WritePidFile.
func ReadPidFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}
