package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/terabiome/homonculus/contracts"
	"github.com/terabiome/homonculus/templator"
	"github.com/urfave/cli/v2"
)

func main() {
	vmTemplator, err := templator.NewLibvirtTemplator("./templates/libvirt-domain.xml.tpl")
	if err != nil {
		log.Fatalf("could not create templator from template file: %v", err)
	}

	app := &cli.App{
		Name:                 "homonculus",
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name:  "virtualmachine",
				Usage: "Execute VM-related functions",
				Action: func(c *cli.Context) error {
					fmt.Println("use subcommand instead:")
					for _, subcmd := range c.Command.Subcommands {
						fmt.Printf("\t - %s %s %s\n", c.App.Name, c.Command.Name, subcmd.Name)
					}
					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "create",
						Usage: "Create virtual machine(s)",
						Action: func(ctx *cli.Context) error {
							filepath := ctx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							rawContent, err := os.ReadFile(filepath)
							if err != nil {
								return fmt.Errorf("could not read virtualmachine config: %v (path = %v)", err, filepath)
							}

							var clusterRequest contracts.ClusterRequest
							err = json.Unmarshal(rawContent, &clusterRequest)
							if err != nil {
								return fmt.Errorf("could not deserialize virtualmachine config: %v", err)
							}

							for _, virtualMachine := range clusterRequest.VirtualMachines {
								placeholders := templator.LibvirtTemplatePlaceholder{
									Name:             virtualMachine.Name,
									UUID:             uuid.New(),
									VCPU:             virtualMachine.VCPU,
									MemoryKiB:        virtualMachine.MemoryMB << 10,
									DiskPath:         virtualMachine.DiskPath,
									CloudInitISOPath: virtualMachine.CloudInitISOPath,
								}

								log.Printf("creating Libvirt XML for VM %s (uuid = %v) ...", placeholders.Name, placeholders.UUID)
								err = vmTemplator.ToFile(fmt.Sprintf("./libvirt-%s.xml", virtualMachine.Name), placeholders)
								if err != nil {
									log.Printf("unable to create Libvirt XML for VM %s (uuid = %v): %v", placeholders.Name, placeholders.UUID, err)
									continue
								}

								log.Printf("creating QCOW2 disk for VM %s (uuid = %v) at path %s (%d GB)...",
									placeholders.Name,
									placeholders.UUID,
									virtualMachine.DiskPath,
									virtualMachine.DiskSizeGB,
								)

								if err := createQcow2Disk(virtualMachine.DiskPath, virtualMachine.DiskSizeGB); err != nil {
									log.Printf("unable to create QCOW2 disk for VM %s (uuid = %v): %s", placeholders.Name, placeholders.UUID, err)
									continue
								}

								log.Printf("created VM %s (uuid = %v) ...", placeholders.Name, placeholders.UUID)
							}
							return nil
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		panic(err)
	}
}

func createQcow2Disk(diskPath string, sizeGB int64) error {
	dir := filepath.Dir(diskPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("could not create parent directory %s: %w", dir, err)
	}

	sizeStr := fmt.Sprintf("%dG", sizeGB)
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", diskPath, sizeStr)
	log.Printf("executing: %s", cmd.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img failed: %w - %s", err, string(output))
	}
	return nil
}
