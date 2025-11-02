package disk

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/pkg/executor"
)

type Service struct {
	logger *slog.Logger
}

func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With(slog.String("service", "disk")),
	}
}

func (s *Service) CreateDisk(ctx context.Context, req contracts.CreateVirtualMachineRequest) error {
	s.logger.Debug("creating qcow2 disk",
		slog.String("path", req.DiskPath),
		slog.String("base", req.BaseImagePath),
		slog.Int64("size_gb", req.DiskSizeGB),
	)

	backingFileFormat := strings.ToLower(path.Ext(req.BaseImagePath))
	switch backingFileFormat {
	case ".qcow2":
		backingFileFormat = "qcow2"
	default:
		return fmt.Errorf("unsupported backing file format: %s", backingFileFormat)
	}

	args := []string{
		"create",
		"-b", req.BaseImagePath,
		"-F", backingFileFormat,
		"-f", "qcow2",
		req.DiskPath,
		fmt.Sprintf("%dG", req.DiskSizeGB),
	}

	result, err := executor.RunAndCapture(ctx, req.Executor, "qemu-img", args...)
	if err != nil {
		return fmt.Errorf("qemu-img failed: %w\nstdout: %s\nstderr: %s",
			err, result.Stdout, result.Stderr)
	}

	s.logger.Info("created qcow2 disk",
		slog.String("path", req.DiskPath),
		slog.Int64("size_gb", req.DiskSizeGB),
	)

	return nil
}

func (s *Service) CreateDiskForClone(ctx context.Context, req contracts.TargetVirtualMachineCloneInfo) error {
	s.logger.Debug("creating qcow2 disk for clone",
		slog.String("path", req.DiskPath),
		slog.String("base", req.BaseImagePath),
		slog.Int64("size_gb", req.DiskSizeGB),
	)

	backingFileFormat := strings.ToLower(path.Ext(req.BaseImagePath))
	switch backingFileFormat {
	case ".qcow2":
		backingFileFormat = "qcow2"
	default:
		return fmt.Errorf("unsupported backing file format: %s", backingFileFormat)
	}

	args := []string{
		"create",
		"-b", req.BaseImagePath,
		"-F", backingFileFormat,
		"-f", "qcow2",
		req.DiskPath,
		fmt.Sprintf("%dG", req.DiskSizeGB),
	}

	result, err := executor.RunAndCapture(ctx, req.Executor, "qemu-img", args...)
	if err != nil {
		return fmt.Errorf("qemu-img failed: %w\nstdout: %s\nstderr: %s",
			err, result.Stdout, result.Stderr)
	}

	s.logger.Info("created qcow2 disk for clone",
		slog.String("path", req.DiskPath),
		slog.Int64("size_gb", req.DiskSizeGB),
	)

	return nil
}
