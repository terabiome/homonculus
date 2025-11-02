package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/terabiome/homonculus/internal/cloudinit"
	"github.com/terabiome/homonculus/internal/contracts"
	"github.com/terabiome/homonculus/internal/disk"
	"github.com/terabiome/homonculus/internal/libvirt"
	"github.com/terabiome/homonculus/internal/provisioner"
	"github.com/terabiome/homonculus/pkg/templator"
	"github.com/urfave/cli/v2"
)

func main() {
	libvirtTemplator, err := templator.NewLibvirtTemplator("./templates/libvirt/domain.xml.tpl")
	if err != nil {
		log.Fatalf("could not create Libvirt templator from template file: %v", err)
	}

	cloudInitTemplator, err := templator.NewCloudInitTemplator("./templates/cloudinit/user-data.tpl")
	if err != nil {
		log.Fatalf("could not create cloud-init templator from template file: %v", err)
	}

	provisionerService := provisioner.NewService(
		disk.NewService(),
		cloudinit.NewService(cloudInitTemplator),
		libvirt.NewService(libvirtTemplator),
	)

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

							f, _ := os.Open(filepath)

							var clusterRequest contracts.CreateVirtualMachineClusterRequest
							err = json.NewDecoder(f).Decode(&clusterRequest)

							err := provisionerService.CreateCluster(clusterRequest)
							if err != nil {
								return fmt.Errorf("unable to create virtual machines from template data: %w", err)
							}

							return nil
						},
					},
					{
						Name:  "delete",
						Usage: "Delete virtual machine(s)",
						Action: func(ctx *cli.Context) error {
							filepath := ctx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, _ := os.Open(filepath)

							var clusterRequest contracts.DeleteVirtualMachineClusterRequest
							err = json.NewDecoder(f).Decode(&clusterRequest)

							err := provisionerService.DeleteCluster(clusterRequest)
							if err != nil {
								return fmt.Errorf("unable to delete virtual machines from template data: %w", err)
							}

							return nil
						},
					},
					{
						Name:  "clone",
						Usage: "Clone virtual machine(s)",
						Action: func(ctx *cli.Context) error {
							filepath := ctx.Args().First()
							if filepath == "" {
								return errors.New("empty file path to virtualmachine config")
							}

							f, _ := os.Open(filepath)

							var clusterRequest contracts.CloneVirtualMachineClusterRequest
							err = json.NewDecoder(f).Decode(&clusterRequest)

							err := provisionerService.CloneCluster(clusterRequest)
							if err != nil {
								return fmt.Errorf("unable to clone virtual machines from template data: %w", err)
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
