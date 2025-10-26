#cloud-config

# Hostname and User Configuration
hostname: {{ .Hostname }}
users:
  {{- range .UserConfigs }}
  - name: {{ .Username}}
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: users,wheel
    shell: /bin/bash
    {{- if .SSHAuthorizedKeys }}
    ssh_authorized_keys:
      {{- range .SSHAuthorizedKeys }}
      - {{ .}}
      {{- end }}
    {{- end}}
  {{- end }}
