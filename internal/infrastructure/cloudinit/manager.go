package cloudinit

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/runtime"
	"github.com/terabiome/homonculus/pkg/constants"
	"github.com/terabiome/homonculus/pkg/executor/mkisofs"
	"github.com/terabiome/homonculus/pkg/templator"
)

// Manager manages cloud-init ISO operations.
type Manager struct {
	engine *templator.Engine
	logger *slog.Logger
}

// NewManager creates a new cloud-init manager.
func NewManager(engine *templator.Engine, logger *slog.Logger) *Manager {
	return &Manager{
		engine: engine,
		logger: logger.With(slog.String("component", "cloudinit")),
	}
}

// CreateISO creates a cloud-init ISO from templates.
func (m *Manager) CreateISO(ctx context.Context, hypervisor runtime.HypervisorContext, vmRequest api.CreateVMRequest, instanceID uuid.UUID) error {
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("cloud-init-%s-", vmRequest.Name))
	if err != nil {
		return fmt.Errorf("failed to create temp dir for cloud-init: %w", err)
	}

	userDataPath := filepath.Join(tempDir, "user-data")
	if err := m.renderUserData(userDataPath, vmRequest); err != nil {
		return fmt.Errorf("failed to render user-data: %w", err)
	}
	m.logger.Debug("rendered user-data", slog.String("vm", vmRequest.Name))

	isoFiles := []string{userDataPath}

	if m.engine.HasTemplate(constants.TemplateCloudInitMetaData) {
		metaDataPath := filepath.Join(tempDir, "meta-data")
		if err := m.renderMetaData(metaDataPath, vmRequest, instanceID); err != nil {
			return fmt.Errorf("failed to render meta-data: %w", err)
		}
		isoFiles = append(isoFiles, metaDataPath)
		m.logger.Debug("rendered meta-data", slog.String("vm", vmRequest.Name))
	}

	if m.engine.HasTemplate(constants.TemplateCloudInitNetworkConfig) {
		networkConfigPath := filepath.Join(tempDir, "network-config")
		if err := m.renderNetworkConfig(networkConfigPath, vmRequest); err != nil {
			return fmt.Errorf("failed to render network-config: %w", err)
		}
		isoFiles = append(isoFiles, networkConfigPath)
		m.logger.Debug("rendered network-config", slog.String("vm", vmRequest.Name))
	}

	err = mkisofs.CreateISO(ctx, hypervisor.Executor, mkisofs.ISOOptions{
		OutputFile: vmRequest.CloudInitISOPath,
		VolumeID:   "cidata",
		Files:      isoFiles,
	})
	if err != nil {
		return err
	}

	m.logger.Info("created cloud-init ISO",
		slog.String("vm", vmRequest.Name),
		slog.String("path", vmRequest.CloudInitISOPath),
		slog.Int("files", len(isoFiles)),
	)

	return nil
}

func (m *Manager) renderUserData(path string, vmRequest api.CreateVMRequest) error {
	vars := UserDataTemplateVars{
		Hostname:    vmRequest.Name,
		UserConfigs: vmRequest.UserConfigs,
		Role:        vmRequest.Role,
	}

	return m.engine.RenderToFile(constants.TemplateCloudInitUserData, path, vars)
}

func (m *Manager) renderMetaData(path string, vmRequest api.CreateVMRequest, instanceID uuid.UUID) error {
	vars := MetaDataTemplateVars{
		InstanceID: instanceID.String(),
		Hostname:   vmRequest.Name,
	}

	return m.engine.RenderToFile(constants.TemplateCloudInitMetaData, path, vars)
}

func (m *Manager) renderNetworkConfig(path string, vmRequest api.CreateVMRequest) error {
	vars := NetworkConfigTemplateVars{
		Hostname: vmRequest.Name,
	}

	return m.engine.RenderToFile(constants.TemplateCloudInitNetworkConfig, path, vars)
}
