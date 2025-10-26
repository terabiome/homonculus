package contracts

import "github.com/terabiome/homonculus/pkg/constants"

type VirtualMachineRequest struct {
	Name             string                   `json:"name"`
	VCPU             int                      `json:"vcpu"`
	MemoryMB         int64                    `json:"memory_mb"`
	DiskPath         string                   `json:"disk_path"`
	DiskSizeGB       int64                    `json:"disk_size_gb"`
	BaseImagePath    string                   `json:"base_image_path"`
	CloudInitISOPath string                   `json:"cloud_init_iso_path"`
	Role             constants.KubernetesRole `json:"role"`
	UserConfigs      []UserConfig             `json:"user_configs"`
}

type UserConfig struct {
	Username          string   `json:"username"`
	SSHAuthorizedKeys []string `json:"ssh_authorized_keys"`
}

type ClusterRequest struct {
	VirtualMachines []VirtualMachineRequest `json:"virtual_machines"`
}
