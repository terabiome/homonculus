#!/bin/bash

# Build the homonculus binary
go build -o ./homonculus cmd/*

# Create and start VMs
./homonculus virtualmachine create --start definitions/virtualmachine/your-vm-config.json

# Delete VMs
./homonculus virtualmachine delete definitions/virtualmachine/your-vm-config.json

# Bootstrap K3s master node(s)
./homonculus k3s bootstrap master definitions/k3s/your-master-config.json

# Bootstrap K3s worker node(s)
./homonculus k3s bootstrap worker definitions/k3s/your-worker-config.json

# Check DHCP leases for VMs
sudo virsh net-dhcp-leases --network default

# Convert base image from qcow2 to raw format
qemu-img convert -p -f qcow2 -O raw /path/to/source/base.qcow2 /path/to/destination/base.raw

