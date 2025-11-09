package main

import (
	"github.com/terabiome/homonculus/internal/api"
	"github.com/terabiome/homonculus/internal/service"
)

// adaptCreateCluster converts CLI contract to service params
func adaptCreateCluster(req api.CreateClusterRequest) []service.CreateVMParams {
	params := make([]service.CreateVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = adaptCreateVM(vm)
	}
	return params
}

// adaptCreateVM converts a single VM create request to service params
func adaptCreateVM(vm api.CreateVMRequest) service.CreateVMParams {
	var tuning *service.VMTuning

	// Convert tuning configuration if present
	if vm.Tuning != nil {
		tuning = &service.VMTuning{
			VCPUPins: vm.Tuning.VCPUPins,
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
		UserConfigs:            adaptUserConfigs(vm.UserConfigs),
		Tuning:                 tuning,
	}
}

// adaptDeleteCluster converts CLI contract to service params
func adaptDeleteCluster(req api.DeleteClusterRequest) []service.DeleteVMParams {
	params := make([]service.DeleteVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.DeleteVMParams{
			Name: vm.Name,
		}
	}
	return params
}

// adaptStartCluster converts CLI contract to service params
func adaptStartCluster(req api.StartClusterRequest) []service.StartVMParams {
	params := make([]service.StartVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.StartVMParams{
			Name: vm.Name,
		}
	}
	return params
}

// adaptQueryCluster converts CLI contract to service params
func adaptQueryCluster(req api.QueryClusterRequest) []service.QueryVMParams {
	params := make([]service.QueryVMParams, len(req.VirtualMachines))
	for i, vm := range req.VirtualMachines {
		params[i] = service.QueryVMParams{
			Name: vm.Name,
		}
	}
	return params
}

// adaptVMInfoToAPI converts service VMInfo to API VMInfo
func adaptVMInfoToAPI(vmInfos []service.VMInfo) []api.VMInfo {
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

// adaptCloneCluster converts CLI contract to service params
func adaptCloneCluster(req api.CloneClusterRequest) service.CloneVMParams {
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

// adaptUserConfigs converts API user configs to service user configs
func adaptUserConfigs(configs []api.UserConfig) []service.UserConfig {
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
