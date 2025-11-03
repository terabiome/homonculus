package api

import "github.com/terabiome/homonculus/pkg/constants"

// CreateVMRequest contains the configuration for creating a single virtual machine.
type CreateVMRequest struct {
	Name                   string                   `json:"name"`
	VCPU                   int                      `json:"vcpu"`
	MemoryMB               int64                    `json:"memory_mb"`
	DiskPath               string                   `json:"disk_path"`
	DiskSizeGB             int64                    `json:"disk_size_gb"`
	BaseImagePath          string                   `json:"base_image_path"`
	BridgeNetworkInterface string                   `json:"bridge_network_interface"`
	CloudInitISOPath       string                   `json:"cloud_init_iso_path"`
	Role                   constants.KubernetesRole `json:"role,omitempty"`
	UserConfigs            []UserConfig             `json:"user_configs"`
}

// DeleteVMRequest contains the configuration for deleting a single virtual machine.
type DeleteVMRequest struct {
	Name string `json:"name"`
}

// StartVMRequest contains the configuration for starting a single virtual machine.
type StartVMRequest struct {
	Name string `json:"name"`
}

// BaseVMSpec identifies the base virtual machine to clone from.
type BaseVMSpec struct {
	Name string `json:"name"`
}

// TargetVMSpec contains the configuration for a cloned virtual machine.
// BaseImagePath is populated internally from the base VM's disk and is not part of the API.
type TargetVMSpec struct {
	Name          string `json:"name"`
	VCPU          int    `json:"vcpu"`
	MemoryMB      int64  `json:"memory_mb"`
	DiskPath      string `json:"disk_path"`
	DiskSizeGB    int64  `json:"disk_size_gb"`
	BaseImagePath string `json:"-"`
}

