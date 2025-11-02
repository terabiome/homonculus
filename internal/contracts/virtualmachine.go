package contracts

import "github.com/terabiome/homonculus/pkg/constants"

type UserConfig struct {
	Username          string   `json:"username"`
	SSHAuthorizedKeys []string `json:"ssh_authorized_keys"`
}

type CreateVirtualMachineClusterRequest struct {
	VirtualMachines []CreateVirtualMachineRequest `json:"virtual_machines"`
}

type CreateVirtualMachineRequest struct {
	HypervisorContext

	Name                   string                   `json:"name"`
	VCPU                   int                      `json:"vcpu"`
	MemoryMB               int64                    `json:"memory_mb"`
	DiskPath               string                   `json:"disk_path"`
	DiskSizeGB             int64                    `json:"disk_size_gb"`
	BaseImagePath          string                   `json:"base_image_path"`
	BridgeNetworkInterface string                   `json:"bridge_network_interface"`
	CloudInitISOPath       string                   `json:"cloud_init_iso_path"`
	Role                   constants.KubernetesRole `json:"role"`
	UserConfigs            []UserConfig             `json:"user_configs"`
}
type DeleteVirtualMachineRequest struct {
	HypervisorContext

	Name string `json:"name"`
}

type DeleteVirtualMachineClusterRequest struct {
	VirtualMachines []DeleteVirtualMachineRequest `json:"virtual_machines"`
}

type BaseVirtualMachineCloneInfo struct {
	Name string `json:"name"`
}
type TargetVirtualMachineCloneInfo struct {
	HypervisorContext

	Name          string `json:"name"`
	VCPU          int    `json:"vcpu"`
	MemoryMB      int64  `json:"memory_mb"`
	DiskPath      string `json:"disk_path"`
	DiskSizeGB    int64  `json:"disk_size_gb"`
	BaseImagePath string
}

type CloneVirtualMachineClusterRequest struct {
	BaseVirtualMachine    BaseVirtualMachineCloneInfo     `json:"base_virtual_machine"`
	TargetVirtualMachines []TargetVirtualMachineCloneInfo `json:"target_virtual_machines"`
}
