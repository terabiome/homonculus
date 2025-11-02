package contracts

import (
	"github.com/terabiome/homonculus/pkg/executor"
	"libvirt.org/go/libvirt"
)

type HypervisorContext struct {
	URI      string
	Conn     *libvirt.Connect
	Executor executor.Executor
}

