package libvirt

import "github.com/google/uuid"

// VCPUPin represents a vcpu pinning entry for the domain XML.
type VCPUPin struct {
	VCPU   int
	CPUSet string
}

// NUMAMemory contains NUMA memory tuning configuration.
type NUMAMemory struct {
	Nodeset string
	Mode    string
}

// HostBindMount contains list of mount points from host on virtual machines
type HostBindMount struct {
	SourceDir string
	TargetDir string
}

type LibvirtTemplateVars struct {
	Name                   string
	UUID                   uuid.UUID
	MemoryKiB              int64
	VCPUCount              int
	BridgeNetworkInterface string
	DiskPath               string
	CloudInitISOPath       string
	VCPUPins               []VCPUPin
	HostBindMounts         []HostBindMount
	EmulatorCPUSet         string
	NUMAMemory             *NUMAMemory
}
