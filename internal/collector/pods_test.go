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

func newPod(namespace, name string, containers []corev1.Container, initContainers []corev1.Container) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: corev1.PodSpec{
			Containers:     containers,
			InitContainers: initContainers,
		},
	}
}

func newPodMetrics(namespace, name string, cpuUsage, memUsage string) metricsv1beta1.PodMetrics {
	return metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Containers: []metricsv1beta1.ContainerMetrics{
			{
				Name: "main",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(cpuUsage),
					corev1.ResourceMemory: resource.MustParse(memUsage),
				},
			},
		},
	}
}

func containerWithRequests(name, cpu, mem string) corev1.Container {
	return corev1.Container{
		Name: name,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(mem),
			},
		},
	}
}

func TestPods_BasicUsageAndRequests(t *testing.T) {
	clientset := fakeclientset.NewSimpleClientset(
		newPod("default", "web", []corev1.Container{containerWithRequests("web", "100m", "200Mi")}, nil),
	)
	metricsClient := newMetricsClient(nil, []metricsv1beta1.PodMetrics{
		newPodMetrics("default", "web", "50m", "100Mi"),
	})

	usages, assessments, err := Pods(context.Background(), clientset, metricsClient, "default", false)
	if err != nil {
		t.Fatalf("Pods() error = %v", err)
	}
	if len(usages) != 1 || len(assessments) != 1 {
		t.Fatalf("Pods() returned %d usages, %d assessments, want 1 each", len(usages), len(assessments))
	}

	u := usages[0]
	if u.Namespace != "default" || u.Name != "web" {
		t.Fatalf("usage = %+v, want default/web", u)
	}
	if u.CPUUsedMilli != 50 || u.CPURequestMilli != 100 || !u.HasCPURequest {
		t.Errorf("CPU usage/request = %d/%d (has=%v), want 50/100 (true)", u.CPUUsedMilli, u.CPURequestMilli, u.HasCPURequest)
	}
	if u.CPUPercent == nil || *u.CPUPercent != 50 {
		t.Errorf("CPUPercent = %v, want 50", u.CPUPercent)
	}
	wantMemUsed := int64(100 * 1024 * 1024)
	wantMemReq := int64(200 * 1024 * 1024)
	if u.MemUsedBytes != wantMemUsed || u.MemRequestBytes != wantMemReq || !u.HasMemRequest {
		t.Errorf("Mem usage/request = %d/%d (has=%v), want %d/%d (true)", u.MemUsedBytes, u.MemRequestBytes, u.HasMemRequest, wantMemUsed, wantMemReq)
	}

	a := assessments[0]
	if a.CPUUsageMilli != 50 || a.CPURequestMilli != 100 || !a.HasCPURequest {
		t.Errorf("assessment CPU = %+v, want usage=50 request=100 has=true", a)
	}
}

func TestPods_NoRequestsSet(t *testing.T) {
	clientset := fakeclientset.NewSimpleClientset(
		newPod("default", "bare", []corev1.Container{{Name: "bare"}}, nil),
	)
	metricsClient := newMetricsClient(nil, []metricsv1beta1.PodMetrics{
		newPodMetrics("default", "bare", "10m", "20Mi"),
	})

	usages, assessments, err := Pods(context.Background(), clientset, metricsClient, "default", false)
	if err != nil {
		t.Fatalf("Pods() error = %v", err)
	}

	u := usages[0]
	if u.HasCPURequest || u.HasMemRequest {
		t.Errorf("usage = %+v, want HasCPURequest=false HasMemRequest=false", u)
	}
	if u.CPUPercent != nil || u.MemPercent != nil {
		t.Errorf("usage = %+v, want nil percentages when no request is set", u)
	}

	a := assessments[0]
	if a.HasCPURequest || a.HasMemRequest {
		t.Errorf("assessment = %+v, want no requests set", a)
	}
}

func TestPods_AllNamespacesSkipsKubeSystem(t *testing.T) {
	clientset := fakeclientset.NewSimpleClientset(
		newPod("default", "app", []corev1.Container{containerWithRequests("app", "100m", "100Mi")}, nil),
		newPod("kube-system", "coredns", []corev1.Container{containerWithRequests("coredns", "100m", "100Mi")}, nil),
	)
	metricsClient := newMetricsClient(nil, []metricsv1beta1.PodMetrics{
		newPodMetrics("default", "app", "50m", "50Mi"),
		newPodMetrics("kube-system", "coredns", "50m", "50Mi"),
	})

	usages, assessments, err := Pods(context.Background(), clientset, metricsClient, metav1.NamespaceAll, true)
	if err != nil {
		t.Fatalf("Pods() error = %v", err)
	}
	if len(usages) != 1 || usages[0].Namespace != "default" {
		t.Fatalf("usages = %+v, want only the default namespace pod", usages)
	}
	if len(assessments) != 1 || assessments[0].Namespace != "default" {
		t.Fatalf("assessments = %+v, want only the default namespace pod", assessments)
	}
}

func TestPods_InitContainerOverridesWhenLarger(t *testing.T) {
	// The init container asks for more CPU than the regular container, so
	// the scheduler's effective request (and ours) should be the init
	// container's request, not the regular container's.
	clientset := fakeclientset.NewSimpleClientset(
		newPod("default", "migrator",
			[]corev1.Container{containerWithRequests("app", "50m", "50Mi")},
			[]corev1.Container{containerWithRequests("init", "200m", "10Mi")},
		),
	)
	metricsClient := newMetricsClient(nil, []metricsv1beta1.PodMetrics{
		newPodMetrics("default", "migrator", "10m", "10Mi"),
	})

	usages, _, err := Pods(context.Background(), clientset, metricsClient, "default", false)
	if err != nil {
		t.Fatalf("Pods() error = %v", err)
	}

	u := usages[0]
	if u.CPURequestMilli != 200 {
		t.Errorf("CPURequestMilli = %d, want 200 (init container's request should win)", u.CPURequestMilli)
	}
	// The regular container's memory request (50Mi) is larger than the init
	// container's (10Mi), so it should NOT be overridden.
	wantMemReq := int64(50 * 1024 * 1024)
	if u.MemRequestBytes != wantMemReq {
		t.Errorf("MemRequestBytes = %d, want %d (regular container's request should win)", u.MemRequestBytes, wantMemReq)
	}
}
