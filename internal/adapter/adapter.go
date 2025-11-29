package adapter

import (
	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/service"
)

func AdaptCreateCluster(req api.CreateClusterRequest) []service.CreateVMParams {
	params := make([]service.CreateVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = AdaptCreateVM(vm)
	}
	return params
}

func AdaptCreateVM(vm api.CreateVMRequest) service.CreateVMParams {
	var tuning *service.VMTuning

	// Convert tuning configuration if present
	if vm.Tuning != nil {
		tuning = &service.VMTuning{
			VCPUPins:       vm.Tuning.VCPUPins,
			EmulatorCPUSet: vm.Tuning.EmulatorCPUSet,
		}

		// Convert NUMA memory if present
		if vm.Tuning.NUMAMemory != nil {
			tuning.NUMAMemory = &service.NUMAMemory{
				Nodeset: vm.Tuning.NUMAMemory.Nodeset,
				Mode:    vm.Tuning.NUMAMemory.Mode,
			}
		}
	}

	return service.CreateVMParams{
		Name:                   vm.Name,
		VCPUCount:              vm.VCPUCount,
		MemoryMB:               vm.MemoryMB,
		DiskPath:               vm.DiskPath,
		DiskSizeGB:             vm.DiskSizeGB,
		BaseImagePath:          vm.BaseImagePath,
		BridgeNetworkInterface: vm.BridgeNetworkInterface,
		CloudInitISOPath:       vm.CloudInitISOPath,
		Role:                   string(vm.Role),
		DoPackageUpdate:        vm.DoPackageUpdate,
		DoPackageUpgrade:       vm.DoPackageUpgrade,
		UserConfigs:            AdaptUserConfigs(vm.UserConfigs),
		Runcmds:                vm.Runcmds,
		Tuning:                 tuning,
	}
}

func AdaptDeleteCluster(req api.DeleteClusterRequest) []service.DeleteVMParams {
	params := make([]service.DeleteVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.DeleteVMParams{
			Name: vm.Name,
		}
	}
	return params
}

func AdaptStartCluster(req api.StartClusterRequest) []service.StartVMParams {
	params := make([]service.StartVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.StartVMParams{
			Name: vm.Name,
		}
	}
	return params
}

func AdaptQueryCluster(req api.QueryClusterRequest) []service.QueryVMParams {
	params := make([]service.QueryVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.QueryVMParams{
			Name: vm.Name,
		}
	}
	return params
}

func AdaptVMInfoToAPI(vmInfos []service.VMInfo) []api.VMInfo {
	result := make([]api.VMInfo, len(vmInfos))
	for i, info := range vmInfos {
		disks := make([]api.DiskInfo, len(info.Disks))
		for j, d := range info.Disks {
			disks[j] = api.DiskInfo{
				Path:   d.Path,
				Type:   d.Type,
				Device: d.Device,
				SizeGB: d.SizeGB,
			}
		}
		result[i] = api.VMInfo{
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

func AdaptCloneCluster(req api.CloneClusterRequest) service.CloneVMParams {
	targetSpecs := make([]service.TargetVMSpec, len(req.TargetVMs))
	for i, target := range req.TargetVMs {
		targetSpecs[i] = service.TargetVMSpec{
			Name:          target.Name,
			VCPUCount:     target.VCPUCount,
			MemoryMB:      target.MemoryMB,
			DiskPath:      target.DiskPath,
			DiskSizeGB:    target.DiskSizeGB,
			BaseImagePath: target.BaseImagePath,
		}
	}

	return service.CloneVMParams{
		BaseVMName:  req.BaseVM.Name,
		TargetSpecs: targetSpecs,
	}
}

func AdaptUserConfigs(configs []api.UserConfig) []service.UserConfig {
	result := make([]service.UserConfig, len(configs))
	for i, c := range configs {
		result[i] = service.UserConfig{
			Username:          c.Username,
			SSHAuthorizedKeys: c.SSHAuthorizedKeys,
			Password:          c.Password,
		}
	}
	return result
}
