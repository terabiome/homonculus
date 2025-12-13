package service

// NUMAMemory contains NUMA memory tuning configuration.
type NUMAMemory struct {
	Nodeset string
	Mode    string // strict, preferred, or interleave
}

// VMTuning contains virtual machine performance tuning configuration.
type VMTuning struct {
	VCPUPins       []string
	EmulatorCPUSet string
	NUMAMemory     *NUMAMemory
}

// HostBindMount contains list of mount points from host on virtual machines
type HostBindMount struct {
	SourceDir string
	TargetDir string
}

// CreateVMParams contains transport-agnostic parameters for creating a virtual machine.
type CreateVMParams struct {
	Name                   string
	VCPUCount              int
	MemoryMB               int64
	DiskPath               string
	DiskSizeGB             int64
	BaseImagePath          string
	BridgeNetworkInterface string
	CloudInitISOPath       string
	HostBindMounts         []HostBindMount
	Role                   string
	DoPackageUpdate        bool
	DoPackageUpgrade       bool
	UserConfigs            []UserConfig
	Runcmds                []string
	Tuning                 *VMTuning
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
	VCPUCount  uint
	MemoryMB   uint
	Disks      []DiskInfo
	AutoStart  bool
	Persistent bool
	Hostname   string
	IPAddress  string
}

// CloneVMParams contains transport-agnostic parameters for cloning virtual machines.
type CloneVMParams struct {
	BaseVMName  string
	TargetSpecs []TargetVMSpec
}

// TargetVMSpec contains the configuration for a cloned virtual machine.
type TargetVMSpec struct {
	Name          string
	VCPUCount     int
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
