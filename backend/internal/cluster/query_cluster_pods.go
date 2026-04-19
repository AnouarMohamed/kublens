package cluster

import (
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubelens-backend/internal/model"
)

func (s *Service) PodDetail(ctx context.Context, namespace, name string) (model.PodDetail, error) {
	if snapshot, ok := s.StateSnapshot(ctx); ok {
		if detail, found := podDetailFromState(snapshot, namespace, name); found {
			return detail, nil
		}
	}

	if s.inMockMode() {
		return s.mockPodDetail(namespace, name)
	}

	callCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	pod, err := s.client.CoreV1().Pods(namespace).Get(callCtx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return model.PodDetail{}, ErrNotFound
		}
		return model.PodDetail{}, fmt.Errorf("read pod detail: %w", err)
	}

	detail := model.PodDetail{
		PodSummary: mapPodSummary(*pod),
		NodeName:   pod.Spec.NodeName,
		HostIP:     pod.Status.HostIP,
		PodIP:      pod.Status.PodIP,
		Containers: make([]model.ContainerSpec, 0, len(pod.Spec.Containers)),
		Volumes:    make([]model.NamedVolume, 0, len(pod.Spec.Volumes)),
	}

	for _, container := range pod.Spec.Containers {
		detail.Containers = append(detail.Containers, mapContainerSpec(container))
	}
	for _, volume := range pod.Spec.Volumes {
		detail.Volumes = append(detail.Volumes, model.NamedVolume{Name: volume.Name})
	}

	return detail, nil
}

func (s *Service) PodEvents(ctx context.Context, namespace, name string) []model.K8sEvent {
	if snapshot, ok := s.StateSnapshot(ctx); ok {
		events := podEventsFromState(snapshot, namespace, name)
		if len(events) > 0 {
			return events
		}
	}

	if s.inMockMode() {
		return mockPodEvents(name)
	}

	callCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	list, err := s.client.CoreV1().Events(namespace).List(callCtx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
	})
	if err != nil || len(list.Items) == 0 {
		return mockPodEvents(name)
	}

	events := make([]model.K8sEvent, 0, len(list.Items))
	for _, event := range list.Items {
		lastSeen := event.LastTimestamp.Time
		if lastSeen.IsZero() {
			lastSeen = event.EventTime.Time
		}
		if lastSeen.IsZero() {
			lastSeen = event.CreationTimestamp.Time
		}

		events = append(events, model.K8sEvent{
			Type:          event.Type,
			Reason:        event.Reason,
			Age:           formatAge(lastSeen),
			From:          firstNonEmpty(event.ReportingController, event.Source.Component, "kubernetes"),
			Message:       event.Message,
			Namespace:     event.Namespace,
			Resource:      event.InvolvedObject.Name,
			ResourceKind:  event.InvolvedObject.Kind,
			Count:         event.Count,
			LastTimestamp: formatRFC3339(lastSeen),
		})
	}
	return events
}

func (s *Service) PodLogs(ctx context.Context, namespace, name, container string, lines int) string {
	if s.inMockMode() {
		return mockPodLogs(name)
	}

	callCtx, cancel := s.withTimeout(ctx)
	defer cancel()

	if lines <= 0 {
		lines = 150
	}
	tailLines := int64(lines)
	opts := &corev1.PodLogOptions{TailLines: &tailLines}
	if trimmed := strings.TrimSpace(container); trimmed != "" {
		opts.Container = trimmed
	}
	req := s.client.CoreV1().Pods(namespace).GetLogs(name, opts)
	stream, err := req.Stream(callCtx)
	if err != nil {
		return mockPodLogs(name)
	}
	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil || len(body) == 0 {
		return mockPodLogs(name)
	}
	return string(body)
}

func (s *Service) StreamPodLogs(
	ctx context.Context,
	namespace,
	name,
	container string,
	tailLines int,
	follow bool,
) (io.ReadCloser, error) {
	if s.inMockMode() {
		return io.NopCloser(strings.NewReader(mockPodLogs(name))), nil
	}

	if tailLines <= 0 {
		tailLines = 100
	}
	lines := int64(tailLines)
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &lines,
	}
	if trimmed := strings.TrimSpace(container); trimmed != "" {
		opts.Container = trimmed
	}
	req := s.client.CoreV1().Pods(namespace).GetLogs(name, opts)
	return req.Stream(ctx)
}
