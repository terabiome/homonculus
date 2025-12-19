package adapter

import (
	"github.com/terabiome/homonculus/internal/api/contracts"
	"github.com/terabiome/homonculus/internal/service/parameters"
)

type ServiceParameterAdapter struct{}

func NewServiceParameterAdapter() *ServiceParameterAdapter {
	spAdapter := ServiceParameterAdapter{}
	return &spAdapter
}

func (spAdapter ServiceParameterAdapter) AdaptCreateCluster(req contracts.CreateClusterRequest) []parameters.CreateVM {
	params := make([]parameters.CreateVM, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = spAdapter.AdaptCreateVM(vm)
	}
	return params
}

func (spAdapter ServiceParameterAdapter) AdaptCreateVM(vm contracts.CreateVMRequest) parameters.CreateVM {
	var tuning *parameters.VMTuning

	// Convert tuning configuration if present
	if vm.Tuning != nil {
		tuning = &parameters.VMTuning{
			VCPUPins:       vm.Tuning.VCPUPins,
			EmulatorCPUSet: vm.Tuning.EmulatorCPUSet,
		}

		// Convert NUMA memory if present
		if vm.Tuning.NUMAMemory != nil {
			tuning.NUMAMemory = &parameters.NUMAMemory{
				Nodeset: vm.Tuning.NUMAMemory.Nodeset,
				Mode:    vm.Tuning.NUMAMemory.Mode,
			}
		}
	}

	return parameters.CreateVM{
		Name:                   vm.Name,
		VCPUCount:              vm.VCPUCount,
		MemoryMB:               vm.MemoryMB,
		DiskPath:               vm.DiskPath,
		DiskSizeGB:             vm.DiskSizeGB,
		BaseImagePath:          vm.BaseImagePath,
		BridgeNetworkInterface: vm.BridgeNetworkInterface,
		CloudInitISOPath:       vm.CloudInitISOPath,
		HostBindMounts:         spAdapter.AdaptHostBindMounts(vm.HostBindMounts),
		Role:                   string(vm.Role),
		DoPackageUpdate:        vm.DoPackageUpdate,
		DoPackageUpgrade:       vm.DoPackageUpgrade,
		UserConfigs:            spAdapter.AdaptUserConfigs(vm.UserConfigs),
		Runcmds:                vm.Runcmds,
		Tuning:                 tuning,
	}
}

func (spAdapter ServiceParameterAdapter) AdaptDeleteCluster(req contracts.DeleteClusterRequest) []parameters.DeleteVM {
	params := make([]parameters.DeleteVM, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = parameters.DeleteVM{
			Name: vm.Name,
		}
	}
	return params
}

func (spAdapter ServiceParameterAdapter) AdaptStartCluster(req contracts.StartClusterRequest) []parameters.StartVM {
	params := make([]parameters.StartVM, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = parameters.StartVM{
			Name: vm.Name,
		}
	}
	return params
}

func (spAdapter ServiceParameterAdapter) AdaptQueryCluster(req contracts.QueryClusterRequest) []parameters.QueryVM {
	params := make([]parameters.QueryVM, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = parameters.QueryVM{
			Name: vm.Name,
		}
	}
	return params
}

func (spAdapter ServiceParameterAdapter) AdaptVMInfoToAPI(vmInfos []parameters.VMInfo) []contracts.VMInfo {
	result := make([]contracts.VMInfo, len(vmInfos))
	for i, info := range vmInfos {
		disks := make([]contracts.DiskInfo, len(info.Disks))
		for j, d := range info.Disks {
			disks[j] = contracts.DiskInfo{
				Path:   d.Path,
				Type:   d.Type,
				Device: d.Device,
				SizeGB: d.SizeGB,
			}
		}
		result[i] = contracts.VMInfo{
			Name:       info.Name,
			UUID:       info.UUID,
			State:      info.State,
			VCPUCount:  info.VCPUCount,
			MemoryMB:   info.MemoryMB,
			Disks:      disks,
			AutoStart:  info.AutoStart,
			Persistent: info.Persistent,
			Hostname:   info.Hostname,
			IPAddress:  info.IPAddress,
		}
	}
	return result
}

func (spAdapter ServiceParameterAdapter) AdaptCloneCluster(req contracts.CloneClusterRequest) parameters.CloneVM {
	targetSpecs := make([]parameters.TargetVMSpec, len(req.TargetVMs))
	for i, target := range req.TargetVMs {
		targetSpecs[i] = parameters.TargetVMSpec{
			Name:          target.Name,
			VCPUCount:     target.VCPUCount,
			MemoryMB:      target.MemoryMB,
			DiskPath:      target.DiskPath,
			DiskSizeGB:    target.DiskSizeGB,
			BaseImagePath: target.BaseImagePath,
		}
	}

	return parameters.CloneVM{
		BaseVMName:  req.BaseVM.Name,
		TargetSpecs: targetSpecs,
	}
}

func (spAdapter ServiceParameterAdapter) AdaptUserConfigs(configs []contracts.UserConfig) []parameters.UserConfig {
	result := make([]parameters.UserConfig, len(configs))
	for i, c := range configs {
		result[i] = parameters.UserConfig{
			Username:          c.Username,
			SSHAuthorizedKeys: c.SSHAuthorizedKeys,
			Password:          c.Password,
		}
	}
	return result
}

func (spAdapter ServiceParameterAdapter) AdaptHostBindMounts(configs []contracts.HostBindMount) []parameters.HostBindMount {
	result := make([]parameters.HostBindMount, len(configs))
	for i, c := range configs {
		result[i] = parameters.HostBindMount{
			SourceDir: c.SourceDir,
			TargetDir: c.TargetDir,
		}
	}
	return result
}
