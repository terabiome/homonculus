package disk

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/runtime"
	"github.com/terabiome/homonculus/pkg/executor/qemuimg"
)

// Manager manages disk operations.
type Manager struct {
	logger *slog.Logger
}

// NewManager creates a new disk manager.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger: logger.With(slog.String("component", "disk")),
	}
}

// CreateDisk creates a QCOW2 disk with a backing file.
func (m *Manager) CreateDisk(ctx context.Context, hypervisor runtime.HypervisorContext, req api.CreateVMRequest) error {
	m.logger.Debug("creating qcow2 disk",
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

	m.logger.Info("created qcow2 disk",
		slog.String("path", req.DiskPath),
		slog.Int64("size_gb", req.DiskSizeGB),
	)

	return nil
}

// CreateDiskForClone creates a QCOW2 disk for cloning operations.
func (m *Manager) CreateDiskForClone(ctx context.Context, hypervisor runtime.HypervisorContext, req api.TargetVMSpec) error {
	m.logger.Debug("creating qcow2 disk for clone",
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

	m.logger.Info("created qcow2 disk for clone",
		slog.String("path", req.DiskPath),
		slog.Int64("size_gb", req.DiskSizeGB),
	)

	return nil
}

