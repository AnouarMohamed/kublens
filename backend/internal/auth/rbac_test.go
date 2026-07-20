package auth

import (
	"net/http"
	"testing"
)

func TestRequiredRole(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   Role
	}{
		{name: "viewer get", method: http.MethodGet, path: "/api/pods", want: RoleViewer},
		{name: "assistant post", method: http.MethodPost, path: "/api/assistant", want: RoleViewer},
		{name: "assistant feedback post", method: http.MethodPost, path: "/api/assistant/references/feedback", want: RoleViewer},
		{name: "incident create", method: http.MethodPost, path: "/api/incidents", want: RoleViewer},
		{name: "remediation propose", method: http.MethodPost, path: "/api/remediation/propose", want: RoleViewer},
		{name: "remediation gitops", method: http.MethodPost, path: "/api/remediation/rem-1/gitops", want: RoleViewer},
		{name: "remediation reject", method: http.MethodPost, path: "/api/remediation/rem-1/reject", want: RoleViewer},
		{name: "risk analyze", method: http.MethodPost, path: "/api/risk-guard/analyze", want: RoleViewer},
		{name: "ghost simulation", method: http.MethodPost, path: "/api/ghost/simulations", want: RoleViewer},
		{name: "experimental node telemetry ingest", method: http.MethodPost, path: "/api/experimental/ebpf/nodes", want: RoleOperator},
		{name: "experimental fleet drift proposals", method: http.MethodPost, path: "/api/experimental/fleet-drift/propose", want: RoleOperator},
		{name: "autonomous remediation proposal loop", method: http.MethodPost, path: "/api/experimental/autonomous-remediation/propose", want: RoleOperator},
		{name: "memory write", method: http.MethodPost, path: "/api/memory/runbooks", want: RoleOperator},
		{name: "cluster mutate", method: http.MethodPost, path: "/api/pods", want: RoleOperator},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RequiredRole(tc.method, tc.path); got != tc.want {
				t.Fatalf("RequiredRole(%s, %s) = %v, want %v", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

func TestRequiresWriteGate(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{name: "pods create", method: http.MethodPost, path: "/api/pods", want: true},
		{name: "resource apply", method: http.MethodPut, path: "/api/resources/deployments/ns/name/yaml", want: true},
		{name: "remediation execute", method: http.MethodPost, path: "/api/remediation/rem-1/execute", want: true},
		{name: "autonomous remediation proposal loop", method: http.MethodPost, path: "/api/experimental/autonomous-remediation/propose", want: true},
		{name: "experimental node telemetry ingest", method: http.MethodPost, path: "/api/experimental/ebpf/nodes", want: false},
		{name: "experimental fleet drift proposals", method: http.MethodPost, path: "/api/experimental/fleet-drift/propose", want: false},
		{name: "incident resolve", method: http.MethodPost, path: "/api/incidents/inc-1/resolve", want: false},
		{name: "ghost simulation", method: http.MethodPost, path: "/api/ghost/simulations", want: false},
		{name: "memory create", method: http.MethodPost, path: "/api/memory/runbooks", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RequiresWriteGate(tc.method, tc.path); got != tc.want {
				t.Fatalf("RequiresWriteGate(%s, %s) = %t, want %t", tc.method, tc.path, got, tc.want)
			}
		})
	}
}
