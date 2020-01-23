package main

import (
	"fmt"
	"github.com/function61/deployer/pkg/recursivefilecopy"
	"os"
	"syscall"
)

const (
	shimBinaryMountPoint = "/shim" // shim binary is mounted inside the container in this location
	shimDirectory        = "/shim-work-copy"
)

func launchViaShim(argv []string) error {
	if err := recursivefilecopy.Copy(shimDirectory, "/work"); err != nil {
		return fmt.Errorf("shim copyFiles failed: %v", err)
	}

	if err := syscall.Exec(argv[0], argv, os.Environ()); err != nil {
		return fmt.Errorf("exec failed: %v", err)
	}

	return nil // actually unreachable
}
