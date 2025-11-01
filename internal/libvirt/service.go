package libvirt

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/contracts"

	"github.com/terabiome/homonculus/pkg/templator"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
)

type Service struct {
	libvirtTemplator *templator.LibvirtTemplator
}

func NewService(libvirtTemplator *templator.LibvirtTemplator) *Service {
	return &Service{libvirtTemplator}
}

func (svc *Service) CreateVirtualMachine(request contracts.VirtualMachineRequest, virtualMachineUUID uuid.UUID) error {
	libvirtTemplatePlaceholder := templator.LibvirtTemplatePlaceholder{
		Name:                   request.Name,
		UUID:                   virtualMachineUUID,
		VCPU:                   request.VCPU,
		MemoryKiB:              request.MemoryMB << 10,
		DiskPath:               request.DiskPath,
		CloudInitISOPath:       request.CloudInitISOPath,
		BridgeNetworkInterface: request.BridgeNetworkInterface,
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

func (svc *Service) DeleteVirtualMachine(request contracts.VirtualMachineRequest, virtualMachineUUID uuid.UUID) error {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return fmt.Errorf("could not connect to hypervisor: %w", err)
	}
	log.Println("connected to hypervisor")

	domain, err := conn.LookupDomainByName(request.Name)
	if err != nil {
		return fmt.Errorf("could not look up VM by name: %w", err)
	}
	log.Println("looked up VM by name")

	domainXMLString, err := domain.GetXMLDesc(libvirt.DOMAIN_XML_INACTIVE)
	if err != nil {
		return fmt.Errorf("could not read domain XML: %w", err)
	}
	log.Println("read domain XML")

	domainXML := libvirtxml.Domain{}
	err = domainXML.Unmarshal(domainXMLString)
	if err != nil {
		return fmt.Errorf("could not parse domain XML: %w", err)
	}
	log.Println("parsed domain XML")

	for _, disk := range domainXML.Devices.Disks {
		log.Printf("deleting %v disk for VM %s (uuid = %v)...",
			disk.Driver.Type,
			request.Name,
			virtualMachineUUID,
		)
		cmd := exec.Command(
			"rm", "-f", disk.Source.File.File,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("could not delete disk in VM: %w - %s", err, string(output))
		}
	}

	if state, _, _ := domain.GetState(); state != libvirt.DOMAIN_SHUTOFF {
		if err = domain.Destroy(); err != nil {
			return fmt.Errorf("could not destroy VM: %w", err)
		}
		log.Println("destroyed VM")
	}

	if err = domain.Undefine(); err != nil {
		return fmt.Errorf("could not undefine VM: %w", err)
	}
	log.Println("undefined VM")

	return nil
}
