package cloudinit

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/pkg/templator"
)

type Service struct {
	ciTemplator *templator.CloudInitTemplator
}

func NewService(ciTemplator *templator.CloudInitTemplator) *Service {
	return &Service{ciTemplator}
}

func (svc *Service) CreateISO(vmRequest contracts.VirtualMachineRequest) error {
	dirPath := path.Dir(vmRequest.CloudInitISOPath)

	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("could not mkdir for creating cloud-init ISO: %w", err)
	}

	userDataPath := fmt.Sprintf("%s/user-data", dirPath)

	if err := svc.createUserData(userDataPath, vmRequest); err != nil {
		return fmt.Errorf("could not create user-data YAML for cloud-init ISO: %w", err)
	}

	cmd := exec.Command(
		"mkisofs",
		"-output", vmRequest.CloudInitISOPath,
		"-volid", "cidata",
		"-joliet",
		"-r",
		userDataPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkisofs failed: %w - %s", err, string(output))
	}

	return nil
}

func (svc *Service) createUserData(diskpath string, vmRequest contracts.VirtualMachineRequest) error {
	placeholder := templator.CloudInitTemplatePlaceholder{
		Hostname:    vmRequest.Name,
		UserConfigs: vmRequest.UserConfigs,
		Role:        vmRequest.Role,
	}

	err := svc.ciTemplator.ToFile(diskpath, placeholder)
	if err != nil {
		return fmt.Errorf("could not write user-data to disk: %w", err)
	}

	return nil
}
