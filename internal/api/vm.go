package api

import "github.com/terabiome/homonculus/pkg/constants"

// NUMAMemory contains NUMA memory tuning configuration.
type NUMAMemory struct {
	Nodeset string `json:"nodeset"`        // NUMA node set (e.g., "0", "0-1", "0,2")
	Mode    string `json:"mode,omitempty"` // strict, preferred, or interleave (default: strict)
}

// VMTuning contains virtual machine performance tuning configuration.
type VMTuning struct {
	VCPUPins       []string    `json:"vcpu_pins,omitempty"`       // CPU pinning: list of CPU sets
	EmulatorCPUSet string      `json:"emulator_cpuset,omitempty"` // CPU set for QEMU/KVM emulator threads
	NUMAMemory     *NUMAMemory `json:"numa_memory,omitempty"`     // NUMA memory placement
}

// HostBindMount contains list of mount points from host on virtual machines
type HostBindMount struct {
	SourceDir string `json:"source_dir"`
	TargetDir string `json:"target_dir"`
}

// CreateVMRequest contains the configuration for creating a single virtual machine.
type CreateVMRequest struct {
	Name                   string                   `json:"name"`
	VCPUCount              int                      `json:"vcpu_count"`
	MemoryMB               int64                    `json:"memory_mb"`
	DiskPath               string                   `json:"disk_path"`
	DiskSizeGB             int64                    `json:"disk_size_gb"`
	BaseImagePath          string                   `json:"base_image_path"`
	BridgeNetworkInterface string                   `json:"bridge_network_interface"`
	CloudInitISOPath       string                   `json:"cloud_init_iso_path"`
	HostBindMounts         []HostBindMount          `json:"host_bind_mounts"`
	Role                   constants.KubernetesRole `json:"role,omitempty"`
	DoPackageUpdate        bool                     `json:"do_package_update"`
	DoPackageUpgrade       bool                     `json:"do_package_upgrade"`
	UserConfigs            []UserConfig             `json:"user_configs"`
	Runcmds                []string                 `json:"runcmds"`
	Tuning                 *VMTuning                `json:"tuning,omitempty"` // VM performance tuning
}

// DeleteVMRequest contains the configuration for deleting a single virtual machine.
type DeleteVMRequest struct {
	Name string `json:"name"`
}

// StartVMRequest contains the configuration for starting a single virtual machine.
type StartVMRequest struct {
	Name string `json:"name"`
}

// QueryVMRequest contains the configuration for querying a single virtual machine.
type QueryVMRequest struct {
	Name string `json:"name"`
}

// DiskInfo contains information about a VM disk.
type DiskInfo struct {
	Path   string `json:"path"`
	Type   string `json:"type"`   // e.g., "qcow2", "raw"
	Device string `json:"device"` // e.g., "disk", "cdrom"
	SizeGB int64  `json:"size_gb,omitempty"`
}

// VMInfo contains detailed information about a virtual machine.
type VMInfo struct {
	Name       string     `json:"name"`
	UUID       string     `json:"uuid"`
	State      string     `json:"state"` // running, shutoff, paused, etc. (human-readable for JSON)
	VCPUCount  uint       `json:"vcpu_count"`
	MemoryMB   uint       `json:"memory_mb"`
	Disks      []DiskInfo `json:"disks"`
	AutoStart  bool       `json:"autostart"`
	Persistent bool       `json:"persistent"`
	Hostname   string     `json:"hostname,omitempty"`   // DHCP hostname
	IPAddress  string     `json:"ip_address,omitempty"` // DHCP IP address
}

// BaseVMSpec identifies the base virtual machine to clone from.
type BaseVMSpec struct {
	Name string `json:"name"`
}

// TargetVMSpec contains the configuration for a cloned virtual machine.
// BaseImagePath is populated internally from the base VM's disk and is not part of the API.
type TargetVMSpec struct {
	Name          string `json:"name"`
	VCPUCount     int    `json:"vcpu_count"`
	MemoryMB      int64  `json:"memory_mb"`
	DiskPath      string `json:"disk_path"`
	DiskSizeGB    int64  `json:"disk_size_gb"`
	BaseImagePath string `json:"-"`
}
