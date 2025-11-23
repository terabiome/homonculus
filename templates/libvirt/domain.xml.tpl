<domain type='kvm'>
    <!-- VM Identity -->
    <name>{{ .Name }}</name>
    {{- if .UUID }}
    <uuid>{{ .UUID }}</uuid>
    {{- end }}

    <!-- Resources -->
    <memory unit='KiB'>{{ .MemoryKiB }}</memory>
    <currentMemory unit='KiB'>{{ .MemoryKiB }}</currentMemory>
    <vcpu placement='static'>{{ .VCPUCount }}</vcpu>
    {{- if or .VCPUPins .EmulatorCPUSet }}
    <cputune>
        {{- range .VCPUPins }}
        <vcpupin vcpu='{{ .VCPU }}' cpuset='{{ .CPUSet }}'/>
        {{- end }}
        {{- if .EmulatorCPUSet }}
        <emulatorpin cpuset='{{ .EmulatorCPUSet }}'/>
        {{- end }}
    </cputune>
    {{- end }}
    {{- if .NUMAMemory }}
    <numatune>
        <memory mode='{{ .NUMAMemory.Mode }}' nodeset='{{ .NUMAMemory.Nodeset }}'/>
    </numatune>
    {{- end }}

    <!-- OS and Boot Configuration -->
    <os>
        <type arch='x86_64'>hvm</type>
        <boot dev='hd' />
        <boot dev='cdrom' />
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
            <target dev='vdb' bus='virtio' />
            {{/* <boot order='1'/> */}}
        </disk>
        {{- end }}

        <!-- Cloud-Init ISO (SATA is fine for a CD-ROM) -->
        {{- if .CloudInitISOPath }}
        <disk type='file' device='cdrom'>
            <driver name='qemu' type='raw' />
            <source file='{{ .CloudInitISOPath }}' />
            <target dev='hdc' bus='sata' />
            {{/* <boot order='2'/> */}}
            <readonly />
        </disk>
        {{- end }}

        <!-- Network Interface (VirtIO bridge for high performance) -->
        {{- if .BridgeNetworkInterface }}
        <interface type='bridge'>
            <source bridge='{{ .BridgeNetworkInterface }}' />
            <model type='virtio' />
        </interface>
        {{- end }}


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

        <!-- QEMU guest agent -->
        <channel type='unix'>
            <target type='virtio' name='org.qemu.guest_agent.0'/>
        </channel>

    </devices>
</domain>
