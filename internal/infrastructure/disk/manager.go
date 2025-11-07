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

	backingFileFormat, err := parseBackingFileFormat(req.BaseImagePath)
	if err != nil {
		return err
	}

	outputFileFormat, err := parseOutputFileFormat(req.DiskPath)
	if err != nil {
		return err
	}

	err = qemuimg.CreateBackingImage(ctx, hypervisor.Executor, qemuimg.BackingImageOptions{
		BackingFile:       req.BaseImagePath,
		BackingFileFormat: backingFileFormat,
		OutputFile:        req.DiskPath,
		OutputFileFormat:  outputFileFormat,
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

	backingFileFormat, err := parseBackingFileFormat(req.BaseImagePath)
	if err != nil {
		return err
	}

	outputFileFormat, err := parseOutputFileFormat(req.DiskPath)
	if err != nil {
		return err
	}

	err = qemuimg.CreateBackingImage(ctx, hypervisor.Executor, qemuimg.BackingImageOptions{
		BackingFile:       req.BaseImagePath,
		BackingFileFormat: backingFileFormat,
		OutputFile:        req.DiskPath,
		OutputFileFormat:  outputFileFormat,
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

func parseBackingFileFormat(backingFilePath string) (string, error) {
	backingFileFormat := strings.ToLower(path.Ext(backingFilePath))

	switch backingFileFormat {
	case ".qcow2":
		return "qcow2", nil
	}

	return "", fmt.Errorf("unsupported backing file format: %s", backingFileFormat)
}

func parseOutputFileFormat(outputFilePath string) (string, error) {
	outputFileFormat := strings.ToLower(path.Ext(outputFilePath))

	switch outputFileFormat {
	case ".qcow2":
		return "qcow2", nil
	}

	return "", fmt.Errorf("unsupported output file format: %s", outputFileFormat)
}
