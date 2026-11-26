package pkg

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const busyboxURL = "https://www.busybox.net/downloads/binaries/1.31.0-defconfig-multiarch-musl/busybox-x86_64"

// InstallBusyBox puts a busybox binary at <rootfs>/bin/busybox and links /bin/sh to it.
func InstallBusyBox(rootfs string) error {
	binDir := filepath.Join(rootfs, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", binDir, err)
	}

	bb := filepath.Join(binDir, "busybox")
	if _, err := os.Stat(bb); os.IsNotExist(err) {
		if err := downloadBusyBox(bb); err != nil {
			return err
		}
	}

	sh := filepath.Join(binDir, "sh")
	if _, err := os.Lstat(sh); os.IsNotExist(err) {
		if err := os.Symlink("busybox", sh); err != nil {
			return fmt.Errorf("symlink sh: %w", err)
		}
	}
	return nil
}

func downloadBusyBox(dst string) error {
	c := &http.Client{Timeout: 60 * time.Second}
	resp, err := c.Get(busyboxURL)
	if err != nil {
		return fmt.Errorf("download busybox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download busybox: unexpected status %s", resp.Status)
	}

	// write to a temp file, then rename, so a failed download never leaves a broken binary
	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write busybox: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
