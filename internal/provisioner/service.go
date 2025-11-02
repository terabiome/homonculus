package provisioner

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/cloudinit"
	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/internal/disk"
	"github.com/terabiome/homonculus/internal/libvirt"
)

type Service struct {
	diskService      *disk.Service
	cloudinitService *cloudinit.Service
	libvirtService   *libvirt.Service
	logger           *slog.Logger
}

func NewService(
	diskService *disk.Service,
	cloudinitService *cloudinit.Service,
	libvirtService *libvirt.Service,
	logger *slog.Logger,
) *Service {
	return &Service{
		diskService:      diskService,
		cloudinitService: cloudinitService,
		libvirtService:   libvirtService,
		logger:           logger.With(slog.String("service", "provisioner")),
	}
}

func (s *Service) CreateCluster(request contracts.CreateVirtualMachineClusterRequest) error {
	var failedVMs []string

	for _, virtualMachine := range request.VirtualMachines {
		virtualMachineUUID := uuid.New()

		s.logger.Info("creating VM disk",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
			slog.String("path", virtualMachine.DiskPath),
			slog.Int64("size_gb", virtualMachine.DiskSizeGB),
		)

		if err := s.diskService.CreateDisk(virtualMachine.DiskPath, virtualMachine.BaseImagePath, virtualMachine.DiskSizeGB); err != nil {
			s.logger.Error("failed to create disk",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		if virtualMachine.CloudInitISOPath != "" {
			if err := s.cloudinitService.CreateISO(virtualMachine, virtualMachineUUID); err != nil {
				s.logger.Error("failed to create cloud-init ISO",
					slog.String("vm", virtualMachine.Name),
					slog.String("uuid", virtualMachineUUID.String()),
					slog.String("error", err.Error()),
				)
				if err := os.Remove(virtualMachine.DiskPath); err != nil {
					s.logger.Warn("failed to cleanup disk",
						slog.String("path", virtualMachine.DiskPath),
						slog.String("error", err.Error()),
					)
				}
				failedVMs = append(failedVMs, virtualMachine.Name)
				continue
			}
		} else {
			s.logger.Debug("skipping cloud-init ISO creation", slog.String("vm", virtualMachine.Name))
		}

		if err := s.libvirtService.CreateVirtualMachine(virtualMachine, virtualMachineUUID); err != nil {
			s.logger.Error("failed to create VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			if err := os.Remove(virtualMachine.DiskPath); err != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", virtualMachine.DiskPath),
					slog.String("error", err.Error()),
				)
			}
			if virtualMachine.CloudInitISOPath != "" {
				if err := os.Remove(virtualMachine.CloudInitISOPath); err != nil {
					s.logger.Warn("failed to cleanup cloud-init ISO",
						slog.String("path", virtualMachine.CloudInitISOPath),
						slog.String("error", err.Error()),
					)
				}
			}
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		s.logger.Info("successfully created VM",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
		)
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to create %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

func (s *Service) DeleteCluster(request contracts.DeleteVirtualMachineClusterRequest) error {
	var failedVMs []string

	for _, virtualMachine := range request.VirtualMachines {
		s.logger.Info("deleting VM", slog.String("vm", virtualMachine.Name))

		if vmUUID, err := s.libvirtService.DeleteVirtualMachine(virtualMachine); err != nil {
			s.logger.Error("failed to delete VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", vmUUID),
				slog.String("error", err.Error()),
			)
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		s.logger.Info("successfully deleted VM", slog.String("vm", virtualMachine.Name))
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to delete %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

func (s *Service) CloneCluster(request contracts.CloneVirtualMachineClusterRequest) error {
	s.logger.Info("finding base VM for cloning", slog.String("base_vm", request.BaseVirtualMachine.Name))

	baseDomain, err := s.libvirtService.FindVirtualMachine(request.BaseVirtualMachine.Name)
	if err != nil {
		s.logger.Error("failed to find base VM",
			slog.String("base_vm", request.BaseVirtualMachine.Name),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("unable to find base virtual machine %v: %w", request.BaseVirtualMachine.Name, err)
	}

	baseDomainXML, err := s.libvirtService.ToLibvirtXML(baseDomain)
	if err != nil {
		s.logger.Error("failed to get base VM XML",
			slog.String("base_vm", request.BaseVirtualMachine.Name),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("unable to get XML for base virtual machine %v: %w", request.BaseVirtualMachine.Name, err)
	}

	var baseImagePath string
	for _, disk := range baseDomainXML.Devices.Disks {
		if disk.Driver.Type == "qcow2" {
			baseImagePath = disk.Source.File.File
			break
		}
	}

	s.logger.Debug("found base image", slog.String("path", baseImagePath))

	var failedVMs []string

	for _, virtualMachine := range request.TargetVirtualMachines {
		virtualMachineUUID := uuid.New()

		s.logger.Info("cloning VM",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
			slog.String("from", request.BaseVirtualMachine.Name),
		)

		if err := s.diskService.CreateDisk(virtualMachine.DiskPath, baseImagePath, virtualMachine.DiskSizeGB); err != nil {
			s.logger.Error("failed to clone disk",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			if err := os.Remove(virtualMachine.DiskPath); err != nil {
				s.logger.Warn("failed to cleanup disk",
					slog.String("path", virtualMachine.DiskPath),
					slog.String("error", err.Error()),
				)
			}
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		virtualMachine.BaseImagePath = baseImagePath

		if err := s.libvirtService.CloneVirtualMachine(baseDomainXML, virtualMachine, virtualMachineUUID); err != nil {
			s.logger.Error("failed to clone VM",
				slog.String("vm", virtualMachine.Name),
				slog.String("uuid", virtualMachineUUID.String()),
				slog.String("error", err.Error()),
			)
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		s.logger.Info("successfully cloned VM",
			slog.String("vm", virtualMachine.Name),
			slog.String("uuid", virtualMachineUUID.String()),
		)
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to clone %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}
