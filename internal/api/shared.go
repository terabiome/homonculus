package api

// UserConfig represents a user account configuration for cloud-init.
type UserConfig struct {
	Username          string   `json:"username"`
	SSHAuthorizedKeys []string `json:"ssh_authorized_keys"`
	Password          string   `json:"passwd"`
}
