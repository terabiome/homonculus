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

// StartVMParams contains transport-agnostic parameters for starting a virtual machine.
type StartVMParams struct {
	Name string
}

// QueryVMParams contains transport-agnostic parameters for querying a virtual machine.
type QueryVMParams struct {
	Name string
}

// DiskInfo contains information about a VM disk.
type DiskInfo struct {
	Path   string
	Type   string
	Device string
	SizeGB int64
}

// VMInfo contains detailed information about a virtual machine.
type VMInfo struct {
	Name       string
	UUID       string
	State      string
	VCPU       uint
	MemoryMB   uint
	Disks      []DiskInfo
	AutoStart  bool
	Persistent bool
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
	Password          string
}
