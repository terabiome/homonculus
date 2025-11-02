package disk

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type Service struct {
	logger *slog.Logger
}

func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With(slog.String("service", "disk")),
	}
}

func (s *Service) CreateDisk(diskpath, baseImagePath string, sizeGB int64) error {
	s.logger.Debug("creating qcow2 disk",
		slog.String("path", diskpath),
		slog.String("base", baseImagePath),
		slog.Int64("size_gb", sizeGB),
	)

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

	s.logger.Info("created qcow2 disk",
		slog.String("path", diskpath),
		slog.Int64("size_gb", sizeGB),
	)

	return nil
}
