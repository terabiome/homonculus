package disk

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) CreateDisk(path string, sizeGB int64) error {
	log.Printf("Running qemu-img create for %s...", path)

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// The qemu-img command
	sizeStr := fmt.Sprintf("%dG", sizeGB)
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", path, sizeStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img failed: %w - %s", err, string(output))
	}
	return nil
}
