package collector

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func newNodeMetrics(name, cpuUsage, memUsage string) metricsv1beta1.NodeMetrics {
	return metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuUsage),
			corev1.ResourceMemory: resource.MustParse(memUsage),
		},
	}
}

func TestNodes(t *testing.T) {
	clientset := fakeclientset.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			},
		},
	)
	metricsClient := newMetricsClient([]metricsv1beta1.NodeMetrics{
		newNodeMetrics("node-a", "500m", "1Gi"),
		// No matching Node object -> allocatable is unknown, so
		// percentages should come back nil rather than divide by zero.
		newNodeMetrics("node-b", "100m", "256Mi"),
	}, nil)

	got, err := Nodes(context.Background(), clientset, metricsClient)
	if err != nil {
		t.Fatalf("Nodes() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Nodes() returned %d entries, want 2 (%+v)", len(got), got)
	}

	// Sorted alphabetically by name.
	a, b := got[0], got[1]
	if a.Name != "node-a" || b.Name != "node-b" {
		t.Fatalf("Nodes() = %+v, want node-a before node-b", got)
	}

	if a.CPUUsedMilli != 500 || a.CPUAllocatableMilli != 2000 {
		t.Errorf("node-a CPU used/allocatable = %d/%d, want 500/2000", a.CPUUsedMilli, a.CPUAllocatableMilli)
	}
	if a.CPUPercent == nil || *a.CPUPercent != 25 {
		t.Errorf("node-a CPUPercent = %v, want 25", a.CPUPercent)
	}
	wantMemBytes := int64(4 * 1024 * 1024 * 1024)
	if a.MemAllocatableBytes != wantMemBytes {
		t.Errorf("node-a MemAllocatableBytes = %d, want %d", a.MemAllocatableBytes, wantMemBytes)
	}
	if a.MemPercent == nil || *a.MemPercent != 25 {
		t.Errorf("node-a MemPercent = %v, want 25", a.MemPercent)
	}

	if b.CPUAllocatableMilli != 0 || b.CPUPercent != nil {
		t.Errorf("node-b (no matching Node) = %+v, want zero allocatable and nil percent", b)
	}
}
