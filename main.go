package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/templator"
)

func main() {
	vmTemplator, err := templator.New(templator.VIRTUAL_MACHINE)
	if err != nil {
		log.Fatalf("could not create templator from template file: %v", err)
	}

	mapData := map[string]any{
		"Name":             "k3s-master-1",
		"UUID":             uuid.New(),
		"MemoryKiB":        (16 << 10 << 10),
		"VCPU":             4,
		"DiskPath":         "/var/mnt/toshiba_enterprise_mg_8tb_1/libvirt_images/k8s-master-1.qcow2",
		"CloudInitISOPath": "/var/mnt/toshiba_enterprise_mg_8tb_1/cloud_init_images/k8s-master.iso",
	}

	filepath := fmt.Sprintf("./libvirt-%v.xml", mapData["Name"])
	err = vmTemplator.ToFile(filepath, mapData)
	if err != nil {
		log.Fatalf("failed to write template to file: %v", err)
	}
}
