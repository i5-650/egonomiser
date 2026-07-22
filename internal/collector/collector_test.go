package collector

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"e-go-nomiser/internal/sizing"
)

func TestCollect(t *testing.T) {
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
		newPod("default", "idle", []corev1.Container{containerWithRequests("idle", "100m", "100Mi")}, nil),
	)
	metricsClient := newMetricsClient(
		[]metricsv1beta1.NodeMetrics{newNodeMetrics("node-a", "500m", "1Gi")},
		[]metricsv1beta1.PodMetrics{newPodMetrics("default", "idle", "5m", "5Mi")},
	)

	before := time.Now()
	report, err := Collect(context.Background(), clientset, metricsClient, Params{
		Namespace:     "default",
		AllNamespaces: false,
		Sizing: sizing.Params{
			OversizedPct:      20,
			UndersizedPct:     100,
			TargetUtilization: 0.6,
		},
	})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if report.Timestamp.Before(before) {
		t.Errorf("Timestamp = %v, want it set at/after collection start (%v)", report.Timestamp, before)
	}
	if len(report.Nodes) != 1 || report.Nodes[0].Name != "node-a" {
		t.Errorf("Nodes = %+v, want one entry for node-a", report.Nodes)
	}
	if len(report.Pods) != 1 || report.Pods[0].Name != "idle" {
		t.Errorf("Pods = %+v, want one entry for idle", report.Pods)
	}
	// 5m used / 100m requested = 5%, below the 20% oversized threshold.
	if len(report.Sizing.Oversized) != 1 || report.Sizing.Oversized[0].Name != "idle" {
		t.Errorf("Sizing.Oversized = %+v, want the idle pod flagged as oversized", report.Sizing.Oversized)
	}
	if report.Sizing.TargetUtilization != 0.6 {
		t.Errorf("Sizing.TargetUtilization = %v, want 0.6", report.Sizing.TargetUtilization)
	}
}
