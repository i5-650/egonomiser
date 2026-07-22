package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"e-go-nomiser/internal/types"
)

func TestJSON(t *testing.T) {
	report := sampleReport()

	var buf bytes.Buffer
	if err := JSON(&buf, report); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var got types.Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; raw output:\n%s", err, buf.String())
	}

	if !got.Timestamp.Equal(report.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, report.Timestamp)
	}
	if len(got.Nodes) != 1 || got.Nodes[0].Name != "node-a" {
		t.Errorf("Nodes = %+v, want one entry for node-a", got.Nodes)
	}
	if len(got.Pods) != 1 || got.Pods[0].Name != "web" {
		t.Errorf("Pods = %+v, want one entry for web", got.Pods)
	}
	if len(got.Sizing.Undersized) != 1 || got.Sizing.Undersized[0].Name != "hungry" {
		t.Errorf("Sizing.Undersized = %+v, want one entry for hungry", got.Sizing.Undersized)
	}
	if got.Sizing.TargetUtilization != 0.6 {
		t.Errorf("Sizing.TargetUtilization = %v, want 0.6", got.Sizing.TargetUtilization)
	}
}

func TestJSON_OmitsNilPercentFields(t *testing.T) {
	report := types.Report{
		Pods: []types.PodUsage{
			{Namespace: "default", Name: "bare"}, // no requests set -> nil percent pointers
		},
	}

	var buf bytes.Buffer
	if err := JSON(&buf, report); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	pods := raw["pods"].([]interface{})
	pod := pods[0].(map[string]interface{})

	for _, field := range []string{"cpuPercent", "memPercent", "cpuRequestMilli", "memRequestBytes"} {
		if _, ok := pod[field]; ok {
			t.Errorf("pod JSON has field %q, want it omitted (omitempty) when unset: %+v", field, pod)
		}
	}
	if hasCPU, ok := pod["hasCpuRequest"]; !ok || hasCPU != false {
		t.Errorf("pod JSON hasCpuRequest = %v, want false (not omitted, since it's not a pointer)", pod["hasCpuRequest"])
	}
}
