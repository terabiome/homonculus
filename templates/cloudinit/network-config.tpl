version: 2
ethernets:
  eth0:
    {{- if .IPv4Address }}
    dhcp4: false
    addresses:
      - {{ .IPv4Address }}
    {{- if .IPv4GatewayAddress }}
    gateway4: {{ .IPv4GatewayAddress }}
    {{- end }}
    {{ else }}
    dhcp4: true
    {{- end }}
    dhcp6: false
    nameservers:
      addresses: [1.1.1.1, 8.8.8.8]
