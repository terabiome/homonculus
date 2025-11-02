package disk

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) CreateDisk(diskpath, baseImagePath string, sizeGB int64) error {
	log.Printf("Running qemu-img create for %s with base image %s ...", diskpath, baseImagePath)

	dir := filepath.Dir(diskpath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	backingFileFormat := strings.ToLower(path.Ext(baseImagePath))
	switch backingFileFormat {
	case ".qcow2":
		backingFileFormat = "qcow2"
	default:
		return fmt.Errorf("unsupported backing file format: %s", backingFileFormat)
	}

	cmd := exec.Command(
		"qemu-img", "create",
		"-b", baseImagePath,
		"-B", backingFileFormat,
		"-f", "qcow2",
		diskpath, fmt.Sprintf("%dG", sizeGB),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img failed: %w - %s", err, string(output))
	}
	return nil
}
