package runtime

import (
	"github.com/terabiome/homonculus/pkg/executor"
	"libvirt.org/go/libvirt"
)

// HypervisorContext holds runtime dependencies for interacting with a hypervisor.
// These fields are injected by the provisioner and contain active connections and executors.
type HypervisorContext struct {
	URI      string               `json:"-"`
	Conn     *libvirt.Connect     `json:"-"`
	Executor executor.Executor    `json:"-"`
}

