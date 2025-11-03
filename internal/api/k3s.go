package api

// K3sMasterBootstrapConfig contains configuration for bootstrapping K3s master node(s).
type K3sMasterBootstrapConfig struct {
	Nodes []K3sNodeConfig `json:"nodes"`
	Token string          `json:"token"`
}

// K3sWorkerBootstrapConfig contains configuration for bootstrapping K3s worker node(s).
type K3sWorkerBootstrapConfig struct {
	Nodes     []K3sNodeConfig `json:"nodes"`
	Token     string          `json:"token"`
	MasterURL string          `json:"master_url"` // e.g., "https://k3s-master.local:6443" or "https://192.168.122.100:6443"
}

// K3sNodeConfig contains SSH connection details for a node.
type K3sNodeConfig struct {
	Host    string `json:"host"`               // IP address, hostname, or domain
	SSHUser string `json:"ssh_user"`           // SSH username
	SSHKey  string `json:"ssh_key"`            // Path to SSH private key
	SSHPort int    `json:"ssh_port,omitempty"` // SSH port (default: 22)
}
