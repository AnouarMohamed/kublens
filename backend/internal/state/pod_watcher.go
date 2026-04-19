package state

import (
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const recentRestartWindow = 15 * time.Minute

func (c *ClusterCache) onPodAdd(obj any) {
	pod, ok := obj.(*corev1.Pod)
	if !ok || pod == nil {
		return
	}

	info := mapPodInfo(pod)
	key := podKey(info.Namespace, info.Name)

	c.mu.Lock()
	info = c.withRecentRestartSignalsLocked(info, time.Now().UTC())
	prev, existed := c.state.Pods[key]
	c.state.Pods[key] = info
	c.setLastUpdated()
	c.mu.Unlock()

	c.publishPodSignals(prev, info, existed)
}

func (c *ClusterCache) onPodUpdate(oldObj, newObj any) {
	pod, ok := newObj.(*corev1.Pod)
	if !ok || pod == nil {
		return
	}

	info := mapPodInfo(pod)
	key := podKey(info.Namespace, info.Name)

	c.mu.Lock()
	info = c.withRecentRestartSignalsLocked(info, time.Now().UTC())
	prev := c.state.Pods[key]
	c.state.Pods[key] = info
	c.setLastUpdated()
	c.mu.Unlock()

	c.publishPodSignals(prev, info, true)
}

func (c *ClusterCache) onPodDelete(obj any) {
	pod, ok := obj.(*corev1.Pod)
	if !ok || pod == nil {
		return
	}

	key := podKey(pod.Namespace, pod.Name)

	c.mu.Lock()
	delete(c.state.Pods, key)
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("pod_deleted", map[string]any{
		"namespace": pod.Namespace,
		"pod":       pod.Name,
	})
}

func (c *ClusterCache) publishPodSignals(prev PodInfo, current PodInfo, hadPrev bool) {
	c.publish("pod_update", map[string]any{
		"namespace": current.Namespace,
		"pod":       current.Name,
		"status":    current.Phase,
		"restarts":  current.Restarts,
	})

	if hadPrev && current.Restarts > prev.Restarts {
		c.publish("pod_restart", map[string]any{
			"namespace": current.Namespace,
			"pod":       current.Name,
			"restarts":  current.Restarts,
			"reason":    podLastReason(current),
		})
	}

	if hadPrev && !strings.EqualFold(prev.Phase, current.Phase) {
		switch strings.ToLower(current.Phase) {
		case "failed":
			c.publish("pod_failed", map[string]any{
				"namespace": current.Namespace,
				"pod":       current.Name,
				"reason":    podLastReason(current),
			})
		case "pending":
			c.publish("pod_pending", map[string]any{
				"namespace": current.Namespace,
				"pod":       current.Name,
				"reason":    podWaitingReason(current),
			})
		}
	}
}

func podKey(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

func podLastReason(pod PodInfo) string {
	for _, container := range pod.Containers {
		if container.TerminatedReason != "" {
			return container.TerminatedReason
		}
	}
	return pod.StatusReason
}

func podWaitingReason(pod PodInfo) string {
	for _, container := range pod.Containers {
		if container.WaitingReason != "" {
			return container.WaitingReason
		}
	}
	return pod.StatusReason
}

func (c *ClusterCache) withRecentRestartSignalsLocked(info PodInfo, now time.Time) PodInfo {
	info.RecentRestarts = c.countRecentRestartsLocked(info.Namespace, info.Name, now)
	return info
}

func (c *ClusterCache) refreshPodRecentRestartsLocked(namespace, name string, now time.Time) {
	key := podKey(namespace, name)
	pod, ok := c.state.Pods[key]
	if !ok {
		return
	}
	pod.RecentRestarts = c.countRecentRestartsLocked(namespace, name, now)
	c.state.Pods[key] = pod
}

func (c *ClusterCache) countRecentRestartsLocked(namespace, name string, now time.Time) int {
	if namespace == "" || name == "" {
		return 0
	}

	counts := make(map[string]int)
	for _, event := range c.state.Events {
		if event.Namespace != namespace || event.InvolvedObjectName != name {
			continue
		}
		if !isRecentRestartEvent(event, now) {
			continue
		}

		key := event.Namespace + "/" + event.InvolvedObjectName + "|" + event.Reason + "|" + event.Message + "|" + event.Source
		value := normalizedEventCount(event)
		if value > counts[key] {
			counts[key] = value
		}
	}

	total := 0
	for _, value := range counts {
		total += value
	}
	return total
}

func isRecentRestartEvent(event EventInfo, now time.Time) bool {
	if !strings.EqualFold(strings.TrimSpace(event.Type), "Warning") {
		return false
	}

	timestamp := event.LastTimestamp
	if timestamp.IsZero() {
		timestamp = event.FirstTimestamp
	}
	if timestamp.IsZero() || now.Sub(timestamp) > recentRestartWindow {
		return false
	}

	reason := strings.ToLower(strings.TrimSpace(event.Reason))
	message := strings.ToLower(strings.TrimSpace(event.Message))
	for _, fragment := range []string{"backoff", "crashloop", "restart", "killing", "unhealthy", "runcontainererror", "probe"} {
		if strings.Contains(reason, fragment) || strings.Contains(message, fragment) {
			return true
		}
	}

	return false
}

func normalizedEventCount(event EventInfo) int {
	if event.Count > 0 {
		return int(event.Count)
	}
	return 1
}
