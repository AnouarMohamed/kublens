package ghost

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"kubelens-backend/internal/model"
)

func TestGRPCSimulatorIntegration(t *testing.T) {
	// 1. Locate C++ ghost-engine executable
	binPath := "../../../ghost-engine/build/ghost-engine"
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("C++ ghost-engine binary not found, skipping integration test")
	}

	testAddr := "127.0.0.1:8092"

	// 2. Start C++ ghost-engine server
	cmd := exec.Command(binPath, testAddr)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH=../../../third_party/usr/lib64")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start C++ ghost-engine: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Allow server some time to start up and bind to the port
	time.Sleep(500 * time.Millisecond)

	// 3. Create Go gRPC client
	client := NewClient(testAddr, 3*time.Second)

	// 4. Set up sample topology
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	topology := model.GhostTopology{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Nodes: []model.GhostNode{
			{
				Name:          "node-1",
				Status:        "Ready",
				Unschedulable: false,
				Labels:        map[string]string{"kubernetes.io/hostname": "node-1"},
				Headroom:      model.GhostResources{CPUMilli: 500, MemoryBytes: 500},
			},
			{
				Name:          "node-2",
				Status:        "Ready",
				Unschedulable: false,
				Labels:        map[string]string{"kubernetes.io/hostname": "node-2"},
				Headroom:      model.GhostResources{CPUMilli: 1000, MemoryBytes: 1000},
			},
		},
		Pods: []model.GhostPod{
			{
				ID:        "pod-evict",
				Namespace: "default",
				Name:      "pod-evict",
				NodeName:  "node-1",
				Status:    "Running",
				Requests:  model.GhostResources{CPUMilli: 200, MemoryBytes: 200},
			},
		},
	}

	req := model.GhostSimulationRequest{
		Action:         ActionNodeDrain,
		NodeName:       "node-1",
		HorizonSeconds: 600,
	}

	// 5. Run simulation via gRPC client calling C++ engine
	grpcResult, err := client.Simulate(context.Background(), req, topology)
	if err != nil {
		t.Fatalf("gRPC Simulate call failed: %v", err)
	}

	// 6. Run simulation via local Go engine
	localResult := SimulateNodeDrain(topology, req, now)

	// 7. Assert results match
	if grpcResult.Action != localResult.Action {
		t.Errorf("expected Action %q, got %q", localResult.Action, grpcResult.Action)
	}
	if grpcResult.HorizonSeconds != localResult.HorizonSeconds {
		t.Errorf("expected HorizonSeconds %d, got %d", localResult.HorizonSeconds, grpcResult.HorizonSeconds)
	}
	if grpcResult.Verdict.Severity != localResult.Verdict.Severity {
		t.Errorf("expected Severity %q, got %q", localResult.Verdict.Severity, grpcResult.Verdict.Severity)
	}
	if grpcResult.Verdict.Summary != localResult.Verdict.Summary {
		t.Errorf("expected Summary %q, got %q", localResult.Verdict.Summary, grpcResult.Verdict.Summary)
	}

	// Verify pod reschedule in final frame
	if len(grpcResult.Frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(grpcResult.Frames))
	}
	grpcFinalPods := grpcResult.Frames[1].Pods
	localFinalPods := localResult.Frames[1].Pods
	if len(grpcFinalPods) != len(localFinalPods) {
		t.Fatalf("expected %d pods in final frame, got %d", len(localFinalPods), len(grpcFinalPods))
	}
	if grpcFinalPods[0].NodeName != "node-2" {
		t.Errorf("expected pod-evict to move to node-2, got node_name %q", grpcFinalPods[0].NodeName)
	}
}
