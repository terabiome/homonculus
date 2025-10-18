package templator

import "text/template"

const VIRTUAL_MACHINE TemplateName = "virtual-machine"

func loadVirtualMachineTemplate() (*template.Template, error) {
	tmplString := `
		<domain type='kvm'>
			<!-- VM Identity -->
			<name>{{ .Name }}</name>
			<uuid>{{ .UUID }}</uuid>

			<!-- Resources -->
			<memory unit='KiB'>{{ .MemoryKiB }}</memory>
			<currentMemory unit='KiB'>{{ .MemoryKiB }}</currentMemory>
			<vcpu placement='static'>{{ .VCPU }}</vcpu>

			<!-- OS and Boot Configuration -->
			<os>
				<type arch='x86_64' machine='pc-q35-latest'>hvm</type>
				<!-- On first boot, you might boot from a separate install ISO. -->
				<!-- For a running system, boot from the main hard disk. -->
				<boot dev='hd' />
			</os>

			<!-- Performance & Compatibility Features -->
			<features>
				<acpi />
				<apic />
				<vmport state='off' />
			</features>
			<cpu mode='host-passthrough' check='none' migratable='on' />
			<clock offset='utc'>
				<timer name='rtc' tickpolicy='catchup' />
				<timer name='pit' tickpolicy='delay' />
				<timer name='hpet' present='no' />
			</clock>

			<!-- Devices -->
			<devices>
				<!-- Main OS Disk (VirtIO for high performance) -->
				<disk type='file' device='disk'>
					<driver name='qemu' type='qcow2' cache='none' io='native' />
					<source file='{{ .DiskPath }}' />
					<target dev='vda' bus='virtio' />
				</disk>

				<!-- Cloud-Init ISO (SATA is fine for a CD-ROM) -->
				<disk type='file' device='cdrom'>
					<driver name='qemu' type='raw' />
					<source file='{{ .CloudInitISOPath }}' />
					<target dev='hdb' bus='sata' />
					<readonly />
				</disk>

				<!-- Network Interface (VirtIO bridge for high performance) -->
				<interface type='bridge'>
					<source bridge='br0' />
					<model type='virtio' />
				</interface>

				<!-- Essential for headless management -->
				<serial type='pty'>
					<target type='isa-serial' port='0'>
						<model name='isa-serial' />
					</target>
				</serial>
				<console type='pty'>
					<target type='serial' port='0' />
				</console>
			</devices>
		</domain>
	`

	return template.New(string(VIRTUAL_MACHINE)).Parse(tmplString)
}
