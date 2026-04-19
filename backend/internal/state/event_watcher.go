package state

import (
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const maxEventItems = 300

type streamEventPayload struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Age           string `json:"age"`
	From          string `json:"from"`
	Message       string `json:"message"`
	Count         int32  `json:"count,omitempty"`
	LastTimestamp string `json:"lastTimestamp,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	Resource      string `json:"resource,omitempty"`
}

func (c *ClusterCache) onEventAdd(obj any) {
	evt, ok := obj.(*corev1.Event)
	if !ok || evt == nil {
		return
	}

	info := mapEventInfo(evt)
	now := time.Now().UTC()
	c.mu.Lock()
	c.state.Events = append(c.state.Events, info)
	if len(c.state.Events) > maxEventItems {
		c.state.Events = c.state.Events[len(c.state.Events)-maxEventItems:]
	}
	sort.SliceStable(c.state.Events, func(i, j int) bool {
		return c.state.Events[i].LastTimestamp.After(c.state.Events[j].LastTimestamp)
	})
	if strings.EqualFold(info.InvolvedObjectKind, "Pod") {
		c.refreshPodRecentRestartsLocked(info.Namespace, info.InvolvedObjectName, now)
	}
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("k8s_event", c.toStreamEvent(info))
	c.publishSchedulingSignals(info)
}

func (c *ClusterCache) onEventUpdate(oldObj, newObj any) {
	evt, ok := newObj.(*corev1.Event)
	if !ok || evt == nil {
		return
	}
	info := mapEventInfo(evt)
	now := time.Now().UTC()

	c.mu.Lock()
	c.state.Events = append(c.state.Events, info)
	if len(c.state.Events) > maxEventItems {
		c.state.Events = c.state.Events[len(c.state.Events)-maxEventItems:]
	}
	sort.SliceStable(c.state.Events, func(i, j int) bool {
		return c.state.Events[i].LastTimestamp.After(c.state.Events[j].LastTimestamp)
	})
	if strings.EqualFold(info.InvolvedObjectKind, "Pod") {
		c.refreshPodRecentRestartsLocked(info.Namespace, info.InvolvedObjectName, now)
	}
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("k8s_event", c.toStreamEvent(info))
	c.publishSchedulingSignals(info)
}

func (c *ClusterCache) onEventDelete(obj any) {
	evt, ok := obj.(*corev1.Event)
	if !ok || evt == nil {
		return
	}

	now := time.Now().UTC()
	c.mu.Lock()
	trimmed := make([]EventInfo, 0, len(c.state.Events))
	for _, item := range c.state.Events {
		if item.Namespace == evt.Namespace && item.InvolvedObjectName == evt.InvolvedObject.Name && item.Reason == evt.Reason {
			continue
		}
		trimmed = append(trimmed, item)
	}
	c.state.Events = trimmed
	if strings.EqualFold(evt.InvolvedObject.Kind, "Pod") {
		c.refreshPodRecentRestartsLocked(evt.Namespace, evt.InvolvedObject.Name, now)
	}
	c.setLastUpdated()
	c.mu.Unlock()
}

func (c *ClusterCache) publishSchedulingSignals(info EventInfo) {
	reason := strings.ToLower(strings.TrimSpace(info.Reason))
	if reason == "failedscheduling" {
		c.publish("scheduling_failure", map[string]any{
			"namespace": info.Namespace,
			"resource":  info.InvolvedObjectName,
			"reason":    info.Message,
		})
	}
	if strings.Contains(reason, "imagepull") || strings.Contains(reason, "pull") {
		c.publish("image_pull_failure", map[string]any{
			"namespace": info.Namespace,
			"resource":  info.InvolvedObjectName,
			"reason":    info.Message,
		})
	}
}

func (c *ClusterCache) toStreamEvent(info EventInfo) streamEventPayload {
	last := info.LastTimestamp
	if last.IsZero() {
		last = info.FirstTimestamp
	}
	return streamEventPayload{
		Type:          info.Type,
		Reason:        info.Reason,
		Age:           formatAge(last),
		From:          info.Source,
		Message:       info.Message,
		Count:         info.Count,
		LastTimestamp: formatRFC3339(last),
		Namespace:     info.Namespace,
		Resource:      info.InvolvedObjectName,
	}
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	diff := time.Since(t)
	if diff < 0 {
		return "N/A"
	}
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		return formatMinutes(diff)
	}
	if diff < 24*time.Hour {
		return formatHours(diff)
	}
	return formatDays(diff)
}

func formatMinutes(diff time.Duration) string {
	return strconvFormat(int(diff.Minutes()), "m")
}

func formatHours(diff time.Duration) string {
	return strconvFormat(int(diff.Hours()), "h")
}

func formatDays(diff time.Duration) string {
	return strconvFormat(int(diff.Hours()/24), "d")
}

func strconvFormat(value int, suffix string) string {
	return strconv.Itoa(value) + suffix
}

func formatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
