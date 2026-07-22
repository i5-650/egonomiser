// Package k8sconfig builds the Kubernetes REST config used to talk to a
// cluster, preferring in-cluster config and falling back to a kubeconfig
// file on disk.
package k8sconfig

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Load builds a rest.Config, preferring in-cluster config, falling back to
// kubeconfig on disk (same resolution order kubectl-style tools use).
func Load(kubeconfig string) (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// DefaultKubeconfigPath returns "~/.kube/config", the conventional default,
// or "" if the home directory can't be determined.
func DefaultKubeconfigPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
