package provisioner

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/cloudinit"
	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/internal/disk"
	"github.com/terabiome/homonculus/internal/libvirt"
)

type Service struct {
	diskSvc    *disk.Service
	ciSvc      *cloudinit.Service
	libvirtSvc *libvirt.Service
}

func NewService(d *disk.Service, c *cloudinit.Service, l *libvirt.Service) *Service {
	return &Service{diskSvc: d, ciSvc: c, libvirtSvc: l}
}

// CreateCluster uses disk service to create VM disk,
// uses cloud-init service to create cloud-init ISO (templating),
// uses Libvirt service to define domain (templating)
func (s *Service) CreateCluster(request contracts.CreateVirtualMachineClusterRequest) error {
	var failedVMs []string

	for _, virtualMachine := range request.VirtualMachines {
		virtualMachineUUID := uuid.New()

		log.Printf("creating QCOW2 disk for VM %s (uuid = %v) at path %s (%d GB)...",
			virtualMachine.Name,
			virtualMachineUUID,
			virtualMachine.DiskPath,
			virtualMachine.DiskSizeGB,
		)

		if err := s.diskSvc.CreateDisk(virtualMachine.DiskPath, virtualMachine.BaseImagePath, virtualMachine.DiskSizeGB); err != nil {
			log.Printf("unable to create QCOW2 disk for VM %s (uuid = %v): %s", virtualMachine.Name, virtualMachineUUID, err)
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		if virtualMachine.CloudInitISOPath != "" {
			if err := s.ciSvc.CreateISO(virtualMachine); err != nil {
				log.Printf("unable to create cloud-init ISO for VM %s (uuid = %v): %s", virtualMachine.Name, virtualMachineUUID, err)
				log.Printf("cleaning up disk: err = %v", os.Remove(virtualMachine.DiskPath))
				failedVMs = append(failedVMs, virtualMachine.Name)
				continue
			}
		} else {
			log.Printf("skipping cloud-init part due to empty path")
		}

		if err := s.libvirtSvc.CreateVirtualMachine(virtualMachine, virtualMachineUUID); err != nil {
			log.Printf("unable to create VM %s (uuid = %v): %s", virtualMachine.Name, virtualMachineUUID, err)
			log.Printf("cleaning up disk: err = %v", os.Remove(virtualMachine.DiskPath))
			if virtualMachine.CloudInitISOPath != "" {
				log.Printf("cleaning up cloud-init ISO: err = %v", os.Remove(virtualMachine.CloudInitISOPath))
			}
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to create %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

func (s *Service) DeleteCluster(request contracts.DeleteVirtualMachineClusterRequest) error {
	var failedVMs []string

	for _, virtualMachine := range request.VirtualMachines {
		if vmUUID, err := s.libvirtSvc.DeleteVirtualMachine(virtualMachine); err != nil {
			log.Printf("unable to remove VM %s (uuid = %v): %s", virtualMachine.Name, vmUUID, err)
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to delete %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}

func (s *Service) CloneCluster(request contracts.CloneVirtualMachineClusterRequest) error {
	baseDomain, err := s.libvirtSvc.FindVirtualMachine(request.BaseVirtualMachine.Name)
	if err != nil {
		err = fmt.Errorf("unable to find base virtual machine %v: %w",
			request.BaseVirtualMachine.Name,
			err,
		)
		log.Printf("unable to clone virtual machine cluster: %v", err)
		return err
	}

	baseDomainXML, err := s.libvirtSvc.ToLibvirtXML(baseDomain)
	if err != nil {
		err = fmt.Errorf("unable to get XML for base virtual machine %v: %w",
			request.BaseVirtualMachine.Name,
			err,
		)
		log.Printf("unable to clone virtual machine cluster: %v", err)
		return err
	}

	var baseImagePath string
	for _, disk := range baseDomainXML.Devices.Disks {
		if disk.Driver.Type == "qcow2" {
			baseImagePath = disk.Source.File.File
			break
		}
	}

	var failedVMs []string

	for _, virtualMachine := range request.TargetVirtualMachines {
		virtualMachineUUID := uuid.New()

		if err := s.diskSvc.CreateDisk(virtualMachine.DiskPath, baseImagePath, virtualMachine.DiskSizeGB); err != nil {
			log.Printf("unable to clone QCOW2 disk for VM %s (uuid = %v): %s", virtualMachine.Name, virtualMachineUUID, err)
			log.Printf("removing QCOW2 disk: err = %v", os.Remove(virtualMachine.DiskPath))
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}

		virtualMachine.BaseImagePath = baseImagePath

		if err := s.libvirtSvc.CloneVirtualMachine(baseDomainXML, virtualMachine, virtualMachineUUID); err != nil {
			log.Printf("unable to remove VM %s (uuid = %v): %s", virtualMachine.Name, virtualMachineUUID, err)
			failedVMs = append(failedVMs, virtualMachine.Name)
			continue
		}
	}

	if len(failedVMs) > 0 {
		return fmt.Errorf("failed to clone %d VM(s): %v", len(failedVMs), failedVMs)
	}
	return nil
}
