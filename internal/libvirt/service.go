package libvirt

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/contracts"

	"github.com/terabiome/homonculus/pkg/templator"
	"libvirt.org/go/libvirt"
)

type Service struct {
	libvirtTemplator *templator.LibvirtTemplator
}

func NewService(libvirtTemplator *templator.LibvirtTemplator) *Service {
	return &Service{libvirtTemplator}
}

func (svc *Service) CreateVirtualMachine(request contracts.VirtualMachineRequest, virtualMachineUUID uuid.UUID) error {
	libvirtTemplatePlaceholder := templator.LibvirtTemplatePlaceholder{
		Name:             request.Name,
		UUID:             virtualMachineUUID,
		VCPU:             request.VCPU,
		MemoryKiB:        request.MemoryMB << 10,
		DiskPath:         request.DiskPath,
		CloudInitISOPath: request.CloudInitISOPath,
	}

	bytes, err := svc.libvirtTemplator.ToBytes(libvirtTemplatePlaceholder)
	if err != nil {
		return fmt.Errorf("could not create Libvirt XML in memory: %w", err)
	}
	log.Println("created Libvirt XML in memory")

	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return fmt.Errorf("could not connect to hypervisor: %w", err)
	}
	log.Println("connected to hypervisor")

	domain, err := conn.DomainDefineXML(string(bytes))
	if err != nil {
		return fmt.Errorf("could not define VM from Libvirt XML: %w", err)
	}
	log.Println("defined VM from Libvirt XML")

	if err = domain.Create(); err != nil {
		return fmt.Errorf("could not start VM from Libvirt XML: %w", err)
	}
	log.Println("started VM from Libvirt XML")

	return nil
}
