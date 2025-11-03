package api

// CreateClusterRequest contains the configuration for creating a cluster of virtual machines.
type CreateClusterRequest struct {
	VirtualMachines []CreateVMRequest `json:"virtual_machines"`
}

// DeleteClusterRequest contains the configuration for deleting a cluster of virtual machines.
type DeleteClusterRequest struct {
	VirtualMachines []DeleteVMRequest `json:"virtual_machines"`
}

// StartClusterRequest contains the configuration for starting a cluster of virtual machines.
type StartClusterRequest struct {
	VirtualMachines []StartVMRequest `json:"virtual_machines"`
}

// QueryClusterRequest contains the configuration for querying a cluster of virtual machines.
type QueryClusterRequest struct {
	VirtualMachines []QueryVMRequest `json:"virtual_machines"`
}

// QueryClusterResponse contains the response for querying a cluster of virtual machines.
type QueryClusterResponse struct {
	VirtualMachines []VMInfo `json:"virtual_machines"`
}

// CloneClusterRequest contains the configuration for cloning a base VM into multiple target VMs.
type CloneClusterRequest struct {
	BaseVM    BaseVMSpec     `json:"base_virtual_machine"`
	TargetVMs []TargetVMSpec `json:"target_virtual_machines"`
}

