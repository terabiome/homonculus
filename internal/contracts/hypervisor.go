package contracts

import (
	"github.com/terabiome/homonculus/pkg/executor"
	"libvirt.org/go/libvirt"
)

type HypervisorContext struct {
	URI      string            `json:"-"`
	Conn     *libvirt.Connect  `json:"-"`
	Executor executor.Executor `json:"-"`
}
