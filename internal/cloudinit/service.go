package cloudinit

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/pkg/constants"
	"github.com/terabiome/homonculus/pkg/templator"
)

type Service struct {
	engine *templator.Engine
	logger *slog.Logger
}

func NewService(engine *templator.Engine, logger *slog.Logger) *Service {
	return &Service{
		engine: engine,
		logger: logger.With(slog.String("service", "cloudinit")),
	}
}

func (svc *Service) CreateISO(vmRequest contracts.CreateVirtualMachineRequest, instanceID uuid.UUID) error {
	dirPath := filepath.Dir(vmRequest.CloudInitISOPath)

	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("could not create directory for cloud-init ISO: %w", err)
	}

	userDataPath := filepath.Join(dirPath, "user-data")
	if err := svc.renderUserData(userDataPath, vmRequest); err != nil {
		return fmt.Errorf("failed to render user-data: %w", err)
	}
	svc.logger.Debug("rendered user-data", slog.String("vm", vmRequest.Name))

	isoFiles := []string{userDataPath}

	if svc.engine.HasTemplate(constants.TemplateCloudInitMetaData) {
		metaDataPath := filepath.Join(dirPath, "meta-data")
		if err := svc.renderMetaData(metaDataPath, vmRequest, instanceID); err != nil {
			return fmt.Errorf("failed to render meta-data: %w", err)
		}
		isoFiles = append(isoFiles, metaDataPath)
		svc.logger.Debug("rendered meta-data", slog.String("vm", vmRequest.Name))
	}

	if svc.engine.HasTemplate(constants.TemplateCloudInitNetworkConfig) {
		networkConfigPath := filepath.Join(dirPath, "network-config")
		if err := svc.renderNetworkConfig(networkConfigPath, vmRequest); err != nil {
			return fmt.Errorf("failed to render network-config: %w", err)
		}
		isoFiles = append(isoFiles, networkConfigPath)
		svc.logger.Debug("rendered network-config", slog.String("vm", vmRequest.Name))
	}

	args := []string{"-output", vmRequest.CloudInitISOPath, "-volid", "cidata", "-joliet", "-r"}
	args = append(args, isoFiles...)

	cmd := exec.Command("mkisofs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkisofs failed: %w - %s", err, string(output))
	}

	svc.logger.Info("created cloud-init ISO",
		slog.String("vm", vmRequest.Name),
		slog.String("path", vmRequest.CloudInitISOPath),
		slog.Int("files", len(isoFiles)),
	)

	return nil
}

func (svc *Service) renderUserData(path string, vmRequest contracts.CreateVirtualMachineRequest) error {
	vars := UserDataTemplateVars{
		Hostname:    vmRequest.Name,
		UserConfigs: vmRequest.UserConfigs,
		Role:        vmRequest.Role,
	}

	return svc.engine.RenderToFile(constants.TemplateCloudInitUserData, path, vars)
}

func (svc *Service) renderMetaData(path string, vmRequest contracts.CreateVirtualMachineRequest, instanceID uuid.UUID) error {
	vars := MetaDataTemplateVars{
		InstanceID: instanceID.String(),
		Hostname:   vmRequest.Name,
	}

	return svc.engine.RenderToFile(constants.TemplateCloudInitMetaData, path, vars)
}

func (svc *Service) renderNetworkConfig(path string, vmRequest contracts.CreateVirtualMachineRequest) error {
	vars := NetworkConfigTemplateVars{
		Hostname: vmRequest.Name,
	}

	return svc.engine.RenderToFile(constants.TemplateCloudInitNetworkConfig, path, vars)
}
