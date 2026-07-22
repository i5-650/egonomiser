package k8sconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultKubeconfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory available: %v", err)
	}
	want := filepath.Join(home, ".kube", "config")
	if got := DefaultKubeconfigPath(); got != want {
		t.Errorf("DefaultKubeconfigPath() = %q, want %q", got, want)
	}
}

func TestLoad_FromKubeconfigFile(t *testing.T) {
	// Force the in-cluster path to fail so Load falls back to the
	// kubeconfig file, regardless of the environment the test runs in.
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	const kubeconfigYAML = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.invalid:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user: {}
`
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(kubeconfigYAML), 0o600); err != nil {
		t.Fatalf("writing temp kubeconfig: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", path, err)
	}
	if cfg.Host != "https://example.invalid:6443" {
		t.Errorf("cfg.Host = %q, want %q", cfg.Host, "https://example.invalid:6443")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	if _, err := Load(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("Load() with a missing kubeconfig file returned nil error, want an error")
	}
}
