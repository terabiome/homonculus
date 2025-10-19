package contracts

type VirtualMachineRequest struct {
	Name             string `json:"name"`
	VCPU             int    `json:"vcpu"`
	MemoryMB         int64  `json:"memory_mb"`
	DiskPath         string `json:"disk_path"`
	DiskSizeGB       int64  `json:"disk_size_gb"`
	CloudInitISOPath string `json:"cloud_init_iso_path"`
}

type ClusterRequest struct {
	VirtualMachines []VirtualMachineRequest `json:"virtual_machines"`
}
