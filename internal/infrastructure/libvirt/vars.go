package libvirt

import "github.com/google/uuid"

type LibvirtTemplateVars struct {
	Name                   string
	UUID                   uuid.UUID
	MemoryKiB              int64
	VCPU                   int
	BridgeNetworkInterface string
	DiskPath               string
	CloudInitISOPath       string
}

