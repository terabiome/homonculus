package disk

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/pkg/executor/qemuimg"
)

type Service struct {
	logger *slog.Logger
}

func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With(slog.String("service", "disk")),
	}
}

func (s *Service) CreateDisk(ctx context.Context, hypervisor contracts.HypervisorContext, req contracts.CreateVirtualMachineRequest) error {
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

	err := qemuimg.CreateBackingImage(ctx, hypervisor.Executor, qemuimg.BackingImageOptions{
		BackingFile:       req.BaseImagePath,
		BackingFileFormat: backingFileFormat,
		OutputPath:        req.DiskPath,
		SizeGB:            req.DiskSizeGB,
	})
	if err != nil {
		return err
	}

	s.logger.Info("created qcow2 disk",
		slog.String("path", req.DiskPath),
		slog.Int64("size_gb", req.DiskSizeGB),
	)

	return nil
}

func (s *Service) CreateDiskForClone(ctx context.Context, hypervisor contracts.HypervisorContext, req contracts.TargetVirtualMachineCloneInfo) error {
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

	err := qemuimg.CreateBackingImage(ctx, hypervisor.Executor, qemuimg.BackingImageOptions{
		BackingFile:       req.BaseImagePath,
		BackingFileFormat: backingFileFormat,
		OutputPath:        req.DiskPath,
		SizeGB:            req.DiskSizeGB,
	})
	if err != nil {
		return err
	}

	s.logger.Info("created qcow2 disk for clone",
		slog.String("path", req.DiskPath),
		slog.Int64("size_gb", req.DiskSizeGB),
	)

	return nil
}
