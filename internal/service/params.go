package service

// CreateVMParams contains transport-agnostic parameters for creating a virtual machine.
type CreateVMParams struct {
	Name                   string
	VCPU                   int
	MemoryMB               int64
	DiskPath               string
	DiskSizeGB             int64
	BaseImagePath          string
	BridgeNetworkInterface string
	CloudInitISOPath       string
	Role                   string
	UserConfigs            []UserConfig
}

// DeleteVMParams contains transport-agnostic parameters for deleting a virtual machine.
type DeleteVMParams struct {
	Name string
}

// CloneVMParams contains transport-agnostic parameters for cloning virtual machines.
type CloneVMParams struct {
	BaseVMName  string
	TargetSpecs []TargetVMSpec
}

// TargetVMSpec contains the configuration for a cloned virtual machine.
type TargetVMSpec struct {
	Name          string
	VCPU          int
	MemoryMB      int64
	DiskPath      string
	DiskSizeGB    int64
	BaseImagePath string
}

// UserConfig represents a user account configuration for cloud-init.
type UserConfig struct {
	Username          string
	SSHAuthorizedKeys []string
}

