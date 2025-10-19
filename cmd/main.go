package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

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

								log.Printf("creating VM %s (uuid = %v) ...", placeholders.Name, placeholders.UUID)
								err = vmTemplator.ToFile(fmt.Sprintf("./libvirt-%s.xml", virtualMachine.Name), placeholders)
								if err != nil {
									log.Printf("unable to create VM %s (uuid = %v): %v", placeholders.Name, placeholders.UUID, err)
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
