package state

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOnPodAddPopulatesRecentRestartsFromRecentWarningEvents(t *testing.T) {
	cache := NewClusterCache(nil, nil, Config{})
	now := time.Now().UTC()
	cache.state.Events = []EventInfo{
		{
			Type:               "Warning",
			Reason:             "BackOff",
			Message:            "Back-off restarting failed container",
			Namespace:          "prod",
			InvolvedObjectKind: "Pod",
			InvolvedObjectName: "api",
			Count:              2,
			LastTimestamp:      now.Add(-5 * time.Minute),
		},
		{
			Type:               "Warning",
			Reason:             "Unhealthy",
			Message:            "Readiness probe failed",
			Namespace:          "prod",
			InvolvedObjectKind: "Pod",
			InvolvedObjectName: "api",
			Count:              1,
			LastTimestamp:      now.Add(-10 * time.Minute),
		},
		{
			Type:               "Warning",
			Reason:             "FailedScheduling",
			Message:            "0/3 nodes are available: Insufficient memory",
			Namespace:          "prod",
			InvolvedObjectKind: "Pod",
			InvolvedObjectName: "api",
			Count:              4,
			LastTimestamp:      now.Add(-3 * time.Minute),
		},
		{
			Type:               "Warning",
			Reason:             "BackOff",
			Message:            "Back-off restarting failed container",
			Namespace:          "prod",
			InvolvedObjectKind: "Pod",
			InvolvedObjectName: "api",
			Count:              9,
			LastTimestamp:      now.Add(-20 * time.Minute),
		},
	}

	cache.onPodAdd(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "prod",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	})

	got := cache.Snapshot().Pods["prod/api"].RecentRestarts
	if got != 3 {
		t.Fatalf("RecentRestarts = %d, want 3", got)
	}
}

func TestOnEventAddRefreshesRecentRestartsForExistingPods(t *testing.T) {
	cache := NewClusterCache(nil, nil, Config{})
	cache.onPodAdd(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "prod",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	})

	cache.onEventAdd(&corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "api.1234",
			Namespace:         "prod",
			CreationTimestamp: metav1.NewTime(time.Now().UTC()),
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod",
			Name: "api",
		},
		Type:    "Warning",
		Reason:  "BackOff",
		Message: "Back-off restarting failed container",
		Count:   2,
	})

	got := cache.Snapshot().Pods["prod/api"].RecentRestarts
	if got != 2 {
		t.Fatalf("RecentRestarts = %d, want 2", got)
	}
}
