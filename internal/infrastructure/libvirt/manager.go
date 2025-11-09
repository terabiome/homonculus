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

// CreateVirtualMachine creates a virtual machine without starting it.
func (m *Manager) CreateVirtualMachine(ctx context.Context, hypervisor runtime.HypervisorContext, request api.CreateVMRequest, virtualMachineUUID uuid.UUID) error {
	var vcpuPins []VCPUPin
	var numaMemory *NUMAMemory

	// Process tuning configuration if present
	if request.Tuning != nil {
		// Validate CPU pinning configuration
		if len(request.Tuning.VCPUPins) > 0 {
			if len(request.Tuning.VCPUPins) > request.VCPUCount {
				return fmt.Errorf("vcpu_pins length (%d) exceeds vcpu_count (%d)", len(request.Tuning.VCPUPins), request.VCPUCount)
			}
			if len(request.Tuning.VCPUPins) < request.VCPUCount {
				m.logger.Warn("partial CPU pinning detected",
					slog.String("vm", request.Name),
					slog.Int("vcpu_count", request.VCPUCount),
					slog.Int("vcpu_pins", len(request.Tuning.VCPUPins)),
					slog.String("note", "remaining vCPUs will not be pinned"),
				)
			}

			// Convert cpuset strings to VCPUPin structs with explicit vcpu indices
			vcpuPins = make([]VCPUPin, len(request.Tuning.VCPUPins))
			for i, cpuset := range request.Tuning.VCPUPins {
				vcpuPins[i] = VCPUPin{
					VCPU:   i,
					CPUSet: cpuset,
				}
			}
		}

		// Process NUMA memory configuration
		if request.Tuning.NUMAMemory != nil {
			mode := request.Tuning.NUMAMemory.Mode
			// Default to preferred if not specified (sweet default)
			if mode == "" {
				mode = "preferred"
			}
			// Validate mode
			if mode != "strict" && mode != "preferred" && mode != "interleave" {
				return fmt.Errorf("invalid NUMA memory mode '%s': must be 'strict', 'preferred', or 'interleave'", mode)
			}

			numaMemory = &NUMAMemory{
				Nodeset: request.Tuning.NUMAMemory.Nodeset,
				Mode:    mode,
			}
		}
	}

	vars := LibvirtTemplateVars{
		Name:                   request.Name,
		UUID:                   virtualMachineUUID,
		VCPUCount:              request.VCPUCount,
		MemoryKiB:              request.MemoryMB << 10,
		DiskPath:               request.DiskPath,
		CloudInitISOPath:       request.CloudInitISOPath,
		BridgeNetworkInterface: request.BridgeNetworkInterface,
		VCPUPins:               vcpuPins,
		NUMAMemory:             numaMemory,
	}

	bytes, err := m.engine.RenderToBytes(constants.TemplateLibvirt, vars)
	if err != nil {
		return fmt.Errorf("could not create Libvirt XML in memory: %w", err)
	}
	m.logger.Debug("rendered libvirt XML", slog.String("vm", request.Name))

	_, err = hypervisor.Conn.DomainDefineXML(string(bytes))
	if err != nil {
		return fmt.Errorf("could not define VM from Libvirt XML: %w", err)
	}
	m.logger.Info("defined VM in libvirt", slog.String("vm", request.Name))

	return nil
}

// StartVirtualMachine starts a virtual machine by name.
func (m *Manager) StartVirtualMachine(ctx context.Context, hypervisor runtime.HypervisorContext, request api.StartVMRequest) error {
	domain, err := hypervisor.Conn.LookupDomainByName(request.Name)
	if err != nil {
		return fmt.Errorf("could not look up VM by name: %w", err)
	}
	m.logger.Debug("found VM", slog.String("vm", request.Name))

	if err = domain.Create(); err != nil {
		return fmt.Errorf("could not start VM: %w", err)
	}
	m.logger.Info("started VM", slog.String("vm", request.Name))

	return nil
}

// GetVirtualMachineInfo retrieves detailed information about a virtual machine.
func (m *Manager) GetVirtualMachineInfo(ctx context.Context, hypervisor runtime.HypervisorContext, request api.QueryVMRequest) (api.VMInfo, error) {
	domain, err := hypervisor.Conn.LookupDomainByName(request.Name)
	if err != nil {
		return api.VMInfo{}, fmt.Errorf("could not look up VM by name: %w", err)
	}

	// Get UUID
	uuidStr, err := domain.GetUUIDString()
	if err != nil {
		return api.VMInfo{}, fmt.Errorf("could not get VM UUID: %w", err)
	}

	// Get state
	state, _, err := domain.GetState()
	if err != nil {
		return api.VMInfo{}, fmt.Errorf("could not get VM state: %w", err)
	}

	// Get XML description
	domainXMLString, err := domain.GetXMLDesc(libvirt.DOMAIN_XML_INACTIVE)
	if err != nil {
		return api.VMInfo{}, fmt.Errorf("could not read domain XML: %w", err)
	}

	domainXML := libvirtxml.Domain{}
	if err = domainXML.Unmarshal(domainXMLString); err != nil {
		return api.VMInfo{}, fmt.Errorf("could not parse domain XML: %w", err)
	}

	// Extract disk information
	var disks []api.DiskInfo
	for _, disk := range domainXML.Devices.Disks {
		if disk.Source != nil && disk.Source.File != nil {
			diskInfo := api.DiskInfo{
				Path:   disk.Source.File.File,
				Device: disk.Device,
			}
			if disk.Driver != nil {
				diskInfo.Type = disk.Driver.Type
			}
			// Note: Getting actual disk size would require additional system calls
			// We could use qemu-img info, but that's outside libvirt scope
			disks = append(disks, diskInfo)
		}
	}

	// Get autostart status
	autoStart, err := domain.GetAutostart()
	if err != nil {
		m.logger.Warn("could not get autostart status", slog.String("vm", request.Name), slog.String("error", err.Error()))
		autoStart = false
	}

	// Check if persistent
	persistent, err := domain.IsPersistent()
	if err != nil {
		m.logger.Warn("could not get persistent status", slog.String("vm", request.Name), slog.String("error", err.Error()))
		persistent = false
	}

	vmInfo := api.VMInfo{
		Name:       request.Name,
		UUID:       uuidStr,
		State:      domainStateToString(state), // Convert to string for JSON API
		VCPUCount:  domainXML.VCPU.Value,
		MemoryMB:   domainXML.CurrentMemory.Value / 1024, // Convert from KiB to MiB
		Disks:      disks,
		AutoStart:  autoStart,
		Persistent: persistent,
	}

	// Try to get DHCP lease information (hostname and IP)
	// This only works if the VM is running and has acquired a DHCP lease
	if state == libvirt.DOMAIN_RUNNING {
		hostname, err := domain.GetHostname(libvirt.DOMAIN_GET_HOSTNAME_LEASE)
		if err == nil {
			if hostname == "" {
				m.logger.Warn("retrieved empty hostname")
			} else {
				vmInfo.Hostname = hostname
				m.logger.Debug("retrieved hostname from DHCP lease", slog.String("vm", request.Name), slog.String("hostname", hostname))
			}
		} else {
			m.logger.Warn("could not get hostname", slog.String("vm", request.Name), slog.String("error", err.Error()))
		}

		// Get IP addresses from domain interfaces
		ifaces, err := domain.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
		if err == nil {
			if len(ifaces) < 1 {
				m.logger.Warn("retrieved no interface", slog.String("vm", request.Name))
			} else {
				// Get the first non-loopback IPv4 address
				for _, iface := range ifaces {
					for _, addr := range iface.Addrs {
						if addr.Type == libvirt.IP_ADDR_TYPE_IPV4 && addr.Addr != "127.0.0.1" {
							vmInfo.IPAddress = addr.Addr
							m.logger.Debug("retrieved IP address from DHCP lease", slog.String("vm", request.Name), slog.String("ip", addr.Addr))
							break
						}
					}
					if vmInfo.IPAddress != "" {
						break
					}
				}
			}
		} else {
			m.logger.Warn("could not get network interface(s)", slog.String("vm", request.Name), slog.String("error", err.Error()))
		}
	}

	m.logger.Debug("retrieved VM info", slog.String("vm", request.Name), slog.String("state", vmInfo.State))

	return vmInfo, nil
}

// ListAllVirtualMachines retrieves information about all virtual machines.
func (m *Manager) ListAllVirtualMachines(ctx context.Context, hypervisor runtime.HypervisorContext) ([]api.VMInfo, error) {
	// List all domains (both active and inactive)
	domains, err := hypervisor.Conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	if err != nil {
		return nil, fmt.Errorf("could not list domains: %w", err)
	}

	var vmInfos []api.VMInfo
	for _, domain := range domains {
		name, err := domain.GetName()
		if err != nil {
			m.logger.Warn("could not get domain name", slog.String("error", err.Error()))
			continue
		}

		vmInfo, err := m.GetVirtualMachineInfo(ctx, hypervisor, api.QueryVMRequest{Name: name})
		if err != nil {
			m.logger.Warn("could not get VM info", slog.String("vm", name), slog.String("error", err.Error()))
			continue
		}

		vmInfos = append(vmInfos, vmInfo)
	}

	m.logger.Debug("listed all VMs", slog.Int("count", len(vmInfos)))

	return vmInfos, nil
}

// domainStateToString converts libvirt domain state to a readable string.
func domainStateToString(state libvirt.DomainState) string {
	switch state {
	case libvirt.DOMAIN_NOSTATE:
		return "no-state"
	case libvirt.DOMAIN_RUNNING:
		return "running"
	case libvirt.DOMAIN_BLOCKED:
		return "blocked"
	case libvirt.DOMAIN_PAUSED:
		return "paused"
	case libvirt.DOMAIN_SHUTDOWN:
		return "shutdown"
	case libvirt.DOMAIN_SHUTOFF:
		return "shutoff"
	case libvirt.DOMAIN_CRASHED:
		return "crashed"
	case libvirt.DOMAIN_PMSUSPENDED:
		return "suspended"
	default:
		return "unknown"
	}
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
func (m *Manager) FindVirtualMachine(hypervisor runtime.HypervisorContext, name string) (*libvirt.Domain, error) {
	domain, err := hypervisor.Conn.LookupDomainByName(name)
	if err != nil {
		return nil, fmt.Errorf("could not look up VM by name: %w", err)
	}
	m.logger.Debug("found VM", slog.String("vm", name))

	return domain, nil
}

// CheckVirtualMachineExistence checks if a VM exists.
func (m *Manager) CheckVirtualMachineExistence(hypervisor runtime.HypervisorContext, name string) (bool, error) {
	_, err := hypervisor.Conn.LookupDomainByName(name)
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

// CloneVirtualMachine clones a VM from a base domain XML without starting it.
func (m *Manager) CloneVirtualMachine(ctx context.Context, hypervisor runtime.HypervisorContext, baseDomainXML libvirtxml.Domain, targetInfo api.TargetVMSpec, virtualMachineUUID uuid.UUID) error {
	newDomainXML := baseDomainXML
	newDomainXML.Name = targetInfo.Name
	newDomainXML.UUID = virtualMachineUUID.String()
	newDomainXML.VCPU.Value = uint(targetInfo.VCPUCount)
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

	_, err = hypervisor.Conn.DomainDefineXML(newDomainXMLString)
	if err != nil {
		return fmt.Errorf("could not define VM from Libvirt XML: %w", err)
	}
	m.logger.Info("defined cloned VM in libvirt", slog.String("vm", targetInfo.Name))

	return nil
}
