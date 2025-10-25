package constants

type KubernetesRole string

const (
	KUBERNETES_ROLE_MASTER KubernetesRole = "master"
	KUBERNETES_ROLE_WORKER KubernetesRole = "worker"
)
