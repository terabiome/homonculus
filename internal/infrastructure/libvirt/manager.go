package libvirt

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/runtime"
	"github.com/terabiome/homonculus/pkg/constants"
	"github.com/terabiome/homonculus/pkg/executor/fileops"
	"github.com/terabiome/homonculus/pkg/templator"
	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
)

// Manager manages libvirt VM operations.
type Manager struct {
	engine *templator.Engine
	logger *slog.Logger
}

// NewManager creates a new libvirt manager.
func NewManager(engine *templator.Engine, logger *slog.Logger) *Manager {
	return &Manager{
		engine: engine,
		logger: logger.With(slog.String("component", "libvirt")),
	}
}

// CreateVirtualMachine creates and starts a virtual machine.
func (m *Manager) CreateVirtualMachine(ctx context.Context, hypervisor runtime.HypervisorContext, request api.CreateVMRequest, virtualMachineUUID uuid.UUID) error {
	vars := LibvirtTemplateVars{
		Name:                   request.Name,
		UUID:                   virtualMachineUUID,
		VCPU:                   request.VCPU,
		MemoryKiB:              request.MemoryMB << 10,
		DiskPath:               request.DiskPath,
		CloudInitISOPath:       request.CloudInitISOPath,
		BridgeNetworkInterface: request.BridgeNetworkInterface,
	}

	bytes, err := m.engine.RenderToBytes(constants.TemplateLibvirt, vars)
	if err != nil {
		return fmt.Errorf("could not create Libvirt XML in memory: %w", err)
	}
	m.logger.Debug("rendered libvirt XML", slog.String("vm", request.Name))

	domain, err := hypervisor.Conn.DomainDefineXML(string(bytes))
	if err != nil {
		return fmt.Errorf("could not define VM from Libvirt XML: %w", err)
	}
	m.logger.Debug("defined VM in libvirt", slog.String("vm", request.Name))

	if err = domain.Create(); err != nil {
		return fmt.Errorf("could not start VM from Libvirt XML: %w", err)
	}
	m.logger.Info("started VM", slog.String("vm", request.Name))

	return nil
}

// DeleteVirtualMachine stops and removes a virtual machine.
func (m *Manager) DeleteVirtualMachine(ctx context.Context, hypervisor runtime.HypervisorContext, request api.DeleteVMRequest) (string, error) {
	domain, err := hypervisor.Conn.LookupDomainByName(request.Name)
	if err != nil {
		return "", fmt.Errorf("could not look up VM by name: %w", err)
	}
	m.logger.Debug("found VM", slog.String("vm", request.Name))

	domainXMLString, err := domain.GetXMLDesc(libvirt.DOMAIN_XML_INACTIVE)
	if err != nil {
		return "", fmt.Errorf("could not read domain XML: %w", err)
	}

	domainXML := libvirtxml.Domain{}
	err = domainXML.Unmarshal(domainXMLString)
	if err != nil {
		return "", fmt.Errorf("could not parse domain XML: %w", err)
	}

	vmUUID, err := domain.GetUUIDString()
	if err != nil {
		return "", fmt.Errorf("could not get VM UUID: %w", err)
	}

	for _, disk := range domainXML.Devices.Disks {
		m.logger.Debug("deleting disk",
			slog.String("vm", request.Name),
			slog.String("type", disk.Driver.Type),
			slog.String("path", disk.Source.File.File),
		)
		
		if err := fileops.RemoveFile(ctx, hypervisor.Executor, disk.Source.File.File); err != nil {
			m.logger.Warn("failed to delete disk",
				slog.String("vm", request.Name),
				slog.String("path", disk.Source.File.File),
				slog.String("error", err.Error()),
			)
		}
	}

	if state, _, _ := domain.GetState(); state != libvirt.DOMAIN_SHUTOFF {
		if err = domain.Destroy(); err != nil {
			return "", fmt.Errorf("could not destroy VM: %w", err)
		}
		m.logger.Debug("destroyed running VM", slog.String("vm", request.Name))
	}

	if err = domain.Undefine(); err != nil {
		return "", fmt.Errorf("could not undefine VM: %w", err)
	}
	m.logger.Info("undefined VM from libvirt", slog.String("vm", request.Name))

	return vmUUID, nil
}

// FindVirtualMachine looks up a virtual machine by name.
func (m *Manager) FindVirtualMachine(name string) (*libvirt.Domain, error) {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, fmt.Errorf("could not connect to hypervisor: %w", err)
	}
	defer conn.Close()
	m.logger.Debug("connected to hypervisor")

	domain, err := conn.LookupDomainByName(name)
	if err != nil {
		return nil, fmt.Errorf("could not look up VM by name: %w", err)
	}
	m.logger.Debug("found VM", slog.String("vm", name))

	return domain, nil
}

// CheckVirtualMachineExistence checks if a VM exists.
func (m *Manager) CheckVirtualMachineExistence(name string) (bool, error) {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return false, fmt.Errorf("could not connect to hypervisor: %w", err)
	}
	defer conn.Close()

	_, err = conn.LookupDomainByName(name)
	if err != nil {
		if err.(libvirt.Error).Code == libvirt.ERR_NO_DOMAIN {
			return false, nil
		}
		return false, fmt.Errorf("error checking if VM exists: %w", err)
	}

	return true, nil
}

// ToLibvirtXML converts a libvirt domain to parsed XML.
func (m *Manager) ToLibvirtXML(domain *libvirt.Domain) (libvirtxml.Domain, error) {
	domainXML := libvirtxml.Domain{}
	domainXMLString, err := domain.GetXMLDesc(libvirt.DOMAIN_XML_INACTIVE)
	if err != nil {
		return domainXML, fmt.Errorf("could not read domain XML: %w", err)
	}
	m.logger.Debug("read domain XML")

	err = domainXML.Unmarshal(domainXMLString)
	if err != nil {
		return domainXML, fmt.Errorf("could not parse domain XML: %w", err)
	}
	m.logger.Debug("parsed domain XML")
	return domainXML, nil
}

// CloneVirtualMachine clones a VM from a base domain XML.
func (m *Manager) CloneVirtualMachine(ctx context.Context, hypervisor runtime.HypervisorContext, baseDomainXML libvirtxml.Domain, targetInfo api.TargetVMSpec, virtualMachineUUID uuid.UUID) error {
	newDomainXML := baseDomainXML
	newDomainXML.Name = targetInfo.Name
	newDomainXML.UUID = virtualMachineUUID.String()
	newDomainXML.VCPU.Value = uint(targetInfo.VCPU)
	newDomainXML.CurrentMemory.Value = uint(targetInfo.MemoryMB << 10)
	newDomainXML.CurrentMemory.Unit = "KiB"
	newDomainXML.Memory.Value = uint(targetInfo.MemoryMB << 10)
	newDomainXML.Memory.Unit = "KiB"
	for idx, disk := range newDomainXML.Devices.Disks {
		if disk.Driver.Type == "qcow2" {
			disk.Source.File.File = targetInfo.DiskPath
			newDomainXML.Devices.Disks[idx] = disk
			break
		}
	}

	newDomainXMLString, err := newDomainXML.Marshal()
	if err != nil {
		return fmt.Errorf("could not serialize Libvirt XML to string: %w", err)
	}

	domain, err := hypervisor.Conn.DomainDefineXML(newDomainXMLString)
	if err != nil {
		return fmt.Errorf("could not define VM from Libvirt XML: %w", err)
	}
	m.logger.Debug("defined cloned VM in libvirt", slog.String("vm", targetInfo.Name))

	if err = domain.Create(); err != nil {
		return fmt.Errorf("could not start VM from Libvirt XML: %w", err)
	}
	m.logger.Info("started cloned VM", slog.String("vm", targetInfo.Name))

	return nil
}

