#cloud-config

# Hostname and User Configuration
hostname: {{ .Hostname }}
users:
  {{- range .UserConfigs }}
  - name: {{ .Username }}
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: users,wheel
    shell: /bin/bash
    {{- if .Password }}
    passwd: {{ .Password }}
    lock_passwd: false
    {{- end }}
    {{- if .SSHAuthorizedKeys }}
    ssh_authorized_keys:
      {{- range .SSHAuthorizedKeys }}
      - {{ . }}
      {{- end }}
    {{- end}}
  {{- end }}

{{- if .DoPackageUpdate }}
package_update: true
{{- end }}
{{- if .DoPackageUpgrade }}
package_upgrade: true
{{- end }}

runcmd:
  {{- range .Runcmds }}
  - {{ . }}
  {{- end }}
