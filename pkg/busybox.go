package pkg

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func InstallBusyBox(root string) error {
	bb := filepath.Join(root, "bin", "busybox")

	if err := downloadBusyBox(bb); err != nil {
		return err
	}

	// /bin/sh → busybox
	return os.Symlink("busybox", filepath.Join("bin", "sh"))
}

func downloadBusyBox(dst string) error {
	resp, err := http.Get("https://www.busybox.net/downloads/binaries/1.31.0-defconfig-multiarch-musl/busybox-x86_64")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
