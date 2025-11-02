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
	return service.CreateVMParams{
		Name:                   vm.Name,
		VCPU:                   vm.VCPU,
		MemoryMB:               vm.MemoryMB,
		DiskPath:               vm.DiskPath,
		DiskSizeGB:             vm.DiskSizeGB,
		BaseImagePath:          vm.BaseImagePath,
		BridgeNetworkInterface: vm.BridgeNetworkInterface,
		CloudInitISOPath:       vm.CloudInitISOPath,
		Role:                   string(vm.Role),
		UserConfigs:            adaptUserConfigs(vm.UserConfigs),
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

// adaptCloneCluster converts CLI contract to service params
func adaptCloneCluster(req api.CloneClusterRequest) service.CloneVMParams {
	targetSpecs := make([]service.TargetVMSpec, len(req.TargetVMs))
	for i, target := range req.TargetVMs {
		targetSpecs[i] = service.TargetVMSpec{
			Name:          target.Name,
			VCPU:          target.VCPU,
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
		}
	}
	return result
}

