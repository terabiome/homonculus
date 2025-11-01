<domain type='kvm'>
    <!-- VM Identity -->
    <name>{{ .Name }}</name>
    {{- if .UUID }}
    <uuid>{{ .UUID }}</uuid>
    {{- end }}

    <!-- Resources -->
    <memory unit='KiB'>{{ .MemoryKiB }}</memory>
    <currentMemory unit='KiB'>{{ .MemoryKiB }}</currentMemory>
    <vcpu placement='static'>{{ .VCPU }}</vcpu>

    <!-- OS and Boot Configuration -->
    <os>
        <type arch='x86_64'>hvm</type>
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
        {{- if .DiskPath }}
        <disk type='file' device='disk'>
            <driver name='qemu' type='qcow2' cache='none' io='native' />
            <source file='{{ .DiskPath }}' />
            <target dev='vda' bus='virtio' />
        </disk>
        {{- end }}

        <!-- Cloud-Init ISO (SATA is fine for a CD-ROM) -->
        {{- if .CloudInitISOPath }}
        <disk type='file' device='cdrom'>
            <driver name='qemu' type='raw' />
            <source file='{{ .CloudInitISOPath }}' />
            <target dev='hdb' bus='sata' />
            <readonly />
        </disk>
        {{- end }}

        <!-- Network Interface (VirtIO bridge for high performance) -->
        <interface type='bridge'>
            <source bridge='{{ .BridgeNetworkInterface }}' />
            <model type='virtio' />
        </interface>

        <graphics type='vnc' port='5901' autoport='yes' listen='0.0.0.0'>
            <listen type='address' address='0.0.0.0'/>
        </graphics>

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
