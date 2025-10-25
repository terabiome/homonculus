#cloud-config

# Hostname and User Configuration
hostname: {{ .Hostname }}
users:
  {{ range .UserConfigs }}
  - name: {{ .Username}}
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: users,wheel
    shell: /bin/bash
    ssh_authorized_keys:
      {{ range .SSHAuthorizedKeys }}
      - {{ .}}
      {{ end }}
  {{ end }}

# Kernel modules and sysctl params required by K3s/Calico on MicroOS
modules:
  - overlay
  - br_netfilter
write_files:
  - path: /etc/sysctl.d/99-k3s.conf
    permissions: '0644'
    content: |
      net.bridge.bridge-nf-call-iptables  = 1
      net.ipv4.ip_forward                 = 1
      net.bridge.bridge-nf-call-ip6tables = 1

{{- /*
Conditional logic to set up either the K3s server (master) or agent (worker).
*/ -}}
{{ if eq .Role "master" }}

# --- MASTER NODE CONFIGURATION (CALICO) ---
write_files:
  - path: /etc/rancher/k3s/config.yaml
    permissions: '0640'
    content: |
      # K3s server configuration
      token: "{{ .K3SToken }}"
      cluster-init: true
      # Define the Pod IP range. This MUST match the Calico config below.
      cluster-cidr: "10.244.0.0/16"
      # Disable default CNI (flannel) and ingress (traefik)
      flannel-backend: "none"
      disable:
        - traefik
        - servicelb
      # Point to our containerd config
      containerd-config-path: "/etc/containerd/config.toml"
  
  - path: /etc/containerd/config.toml
    permissions: '0644'
    content: |
      # Set systemd as the cgroup driver for containerd
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
        SystemdCgroup = true

  - path: /root/calico-installation.yaml
    permissions: '0644'
    content: |
      # This manifest configures the Calico installation.
      apiVersion: operator.tigera.io/v1
      kind: Installation
      metadata:
        name: default
      spec:
        # Configures Calico networking.
        calicoNetwork:
          # Note: The IP pool below MUST match the cluster-cidr in k3s config.
          ipPools:
          - blockSize: 26
            cidr: 10.244.0.0/16
            encapsulation: VXLANCrossSubnet
            natOutgoing: Enabled
            nodeSelector: all()

runcmd:
  - [ sh, -c, "sysctl -p /etc/sysctl.d/99-k3s.conf" ]
  # Install K3s server. The config.yaml will be picked up automatically.
  - [ sh, -c, "curl -sfL https://get.k3s.io | sh -" ]
  # Enable and start the K3s server service
  - [ systemctl, daemon-reload ]
  - [ systemctl, enable, k3s-server ]
  - [ systemctl, start, k3s-server ]
  
  # --- Install Calico Operator ---
  # We must wait for the k3s.yaml to be available to use kubectl.
  # We export KUBECONFIG to ensure kubectl works in this script.
  - [ sh, -c, "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml; until [ -f $KUBECONFIG ]; do echo 'Waiting for k3s.yaml...'; sleep 1; done; kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.28.0/manifests/tigera-operator.yaml" ]
  
  # --- Apply the Calico Installation Configuration ---
  - [ sh, -c, "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml; kubectl apply -f /root/calico-installation.yaml" ]

{{ else }}

# --- WORKER NODE CONFIGURATION ---
write_files:
  - path: /etc/containerd/config.toml
    permissions: '0644'
    content: |
      # Set systemd as the cgroup driver for containerd
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
        SystemdCgroup = true

runcmd:
  - [ sh, -c, "sysctl -p /etc/sysctl.d/99-k3s.conf" ]
  # Install K3s agent. We pass the master URL and token as environment variables.
  - [ sh, -c, "curl -sfL https://get.k3s.io | K3S_URL=https://{{ .MasterHostname }}:6443 K3S_TOKEN={{ .K3SToken }} sh -s - --containerd-config-path /etc/containerd/config.toml" ]
  # Enable and start the K3s agent service
  - [ systemctl, daemon-reload ]
  - [ systemctl, enable, k3s-agent ]
  - [ systemctl, start, k3s-agent ]

{{ end }}

