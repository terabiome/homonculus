package provisioner

import (
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
func (s *Service) CreateCluster(request contracts.ClusterRequest) error {
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
			log.Printf("removing QCOW2 disk: err = %v", os.Remove(virtualMachine.DiskPath))
			continue
		}

		if virtualMachine.CloudInitISOPath != "" {
			if err := s.ciSvc.CreateISO(virtualMachine); err != nil {
				log.Printf("unable to create cloud-init ISO for VM %s (uuid = %v): %s", virtualMachine.Name, virtualMachineUUID, err)
				continue
			}
		} else {
			log.Printf("skipping cloud-init part due to empty path")
		}

		if err := s.libvirtSvc.CreateVirtualMachine(virtualMachine, virtualMachineUUID); err != nil {
			log.Printf("unable to create VM %s (uuid = %v): %s", virtualMachine.Name, virtualMachineUUID, err)
			continue
		}
	}
	return nil
}

func (s *Service) DeleteCluster(request contracts.ClusterRequest) error {
	for _, virtualMachine := range request.VirtualMachines {
		virtualMachineUUID := uuid.New()

		if err := s.libvirtSvc.DeleteVirtualMachine(virtualMachine, virtualMachineUUID); err != nil {
			log.Printf("unable to remove VM %s (uuid = %v): %s", virtualMachine.Name, virtualMachineUUID, err)
			continue
		}
	}
	return nil
}
