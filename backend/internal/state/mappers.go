package state

import (
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func mapPodInfo(pod *corev1.Pod) PodInfo {
	if pod == nil {
		return PodInfo{}
	}

	var (
		containers []ContainerInfo
		restarts   int32
		reqs       ResourceQuantities
		limits     ResourceQuantities
	)

	for _, container := range pod.Spec.Containers {
		info := mapContainerInfo(container, pod.Status.ContainerStatuses)
		containers = append(containers, info)
		restarts += info.RestartCount
		reqs = addQuantities(reqs, info.ResourceRequests)
		limits = addQuantities(limits, info.ResourceLimits)
	}

	conditions := make([]ConditionInfo, 0, len(pod.Status.Conditions))
	for _, condition := range pod.Status.Conditions {
		conditions = append(conditions, ConditionInfo{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			Reason:             condition.Reason,
			Message:            condition.Message,
			LastTransitionTime: condition.LastTransitionTime.Time,
		})
	}

	start := time.Time{}
	if pod.Status.StartTime != nil {
		start = pod.Status.StartTime.Time
	}
	var deletionTimestamp *time.Time
	if pod.DeletionTimestamp != nil {
		timestamp := pod.DeletionTimestamp.Time
		deletionTimestamp = &timestamp
	}

	return PodInfo{
		UID:               string(pod.UID),
		Name:              pod.Name,
		Namespace:         pod.Namespace,
		Labels:            cloneStringMap(pod.Labels),
		Phase:             string(pod.Status.Phase),
		StatusReason:      pod.Status.Reason,
		StatusMessage:     pod.Status.Message,
		NodeName:          pod.Spec.NodeName,
		NodeSelector:      cloneStringMap(pod.Spec.NodeSelector),
		Tolerations:       mapTolerations(pod.Spec.Tolerations),
		StartTime:         start,
		DeletionTimestamp: deletionTimestamp,
		Restarts:          restarts,
		Conditions:        conditions,
		Containers:        containers,
		QoSClass:          string(pod.Status.QOSClass),
		ResourceRequests:  reqs,
		ResourceLimits:    limits,
	}
}

func mapTolerations(tolerations []corev1.Toleration) []TolerationInfo {
	out := make([]TolerationInfo, 0, len(tolerations))
	for _, item := range tolerations {
		out = append(out, TolerationInfo{
			Key:      item.Key,
			Value:    item.Value,
			Effect:   string(item.Effect),
			Operator: string(item.Operator),
		})
	}
	return out
}

func mapContainerInfo(container corev1.Container, statuses []corev1.ContainerStatus) ContainerInfo {
	info := ContainerInfo{
		Name:  container.Name,
		Image: container.Image,
	}

	for _, status := range statuses {
		if status.Name != container.Name {
			continue
		}

		info.Ready = status.Ready
		info.RestartCount = status.RestartCount

		switch {
		case status.State.Running != nil:
			info.State = "Running"
		case status.State.Waiting != nil:
			info.State = "Waiting"
			info.WaitingReason = status.State.Waiting.Reason
		case status.State.Terminated != nil:
			info.State = "Terminated"
			info.TerminatedReason = status.State.Terminated.Reason
			info.TerminatedExitCode = status.State.Terminated.ExitCode
			info.TerminatedFinishedAt = status.State.Terminated.FinishedAt.Time
		}
		if status.LastTerminationState.Terminated != nil {
			if info.TerminatedReason == "" {
				info.TerminatedReason = status.LastTerminationState.Terminated.Reason
			}
			if info.TerminatedExitCode == 0 {
				info.TerminatedExitCode = status.LastTerminationState.Terminated.ExitCode
			}
			if info.TerminatedFinishedAt.IsZero() {
				info.TerminatedFinishedAt = status.LastTerminationState.Terminated.FinishedAt.Time
			}
		}
		break
	}

	info.ResourceRequests = mapQuantities(container.Resources.Requests)
	info.ResourceLimits = mapQuantities(container.Resources.Limits)
	return info
}

func mapNodeInfo(node *corev1.Node) NodeInfo {
	if node == nil {
		return NodeInfo{}
	}

	conditions := make([]ConditionInfo, 0, len(node.Status.Conditions))
	status := "Unknown"
	for _, condition := range node.Status.Conditions {
		conditions = append(conditions, ConditionInfo{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			Reason:             condition.Reason,
			Message:            condition.Message,
			LastTransitionTime: condition.LastTransitionTime.Time,
		})
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				status = "Ready"
			} else {
				status = "NotReady"
			}
		}
	}

	roles := detectNodeRoles(node.Labels)
	taints := make([]string, 0, len(node.Spec.Taints))
	for _, taint := range node.Spec.Taints {
		taints = append(taints, string(taint.Key))
	}

	return NodeInfo{
		UID:           string(node.UID),
		Name:          node.Name,
		Status:        status,
		Roles:         roles,
		Unschedulable: node.Spec.Unschedulable,
		Version:       node.Status.NodeInfo.KubeletVersion,
		CreatedAt:     node.CreationTimestamp.Time,
		Conditions:    conditions,
		Capacity:      mapQuantities(node.Status.Capacity),
		Allocatable:   mapQuantities(node.Status.Allocatable),
		Labels:        cloneStringMap(node.Labels),
		Taints:        taints,
	}
}

func mapDeploymentInfo(deploy *appsv1.Deployment) DeploymentInfo {
	if deploy == nil {
		return DeploymentInfo{}
	}

	conditions := make([]ConditionInfo, 0, len(deploy.Status.Conditions))
	for _, condition := range deploy.Status.Conditions {
		conditions = append(conditions, ConditionInfo{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			Reason:             condition.Reason,
			Message:            condition.Message,
			LastTransitionTime: condition.LastTransitionTime.Time,
		})
	}

	desired := int32(1)
	if deploy.Spec.Replicas != nil {
		desired = *deploy.Spec.Replicas
	}

	return DeploymentInfo{
		UID:               string(deploy.UID),
		Name:              deploy.Name,
		Namespace:         deploy.Namespace,
		DesiredReplicas:   desired,
		ReadyReplicas:     deploy.Status.ReadyReplicas,
		UpdatedReplicas:   deploy.Status.UpdatedReplicas,
		AvailableReplicas: deploy.Status.AvailableReplicas,
		Strategy:          string(deploy.Spec.Strategy.Type),
		Conditions:        conditions,
		CreatedAt:         deploy.CreationTimestamp.Time,
	}
}

func mapEventInfo(evt *corev1.Event) EventInfo {
	if evt == nil {
		return EventInfo{}
	}

	first := evt.FirstTimestamp.Time
	last := evt.LastTimestamp.Time
	if last.IsZero() {
		last = evt.EventTime.Time
	}
	if last.IsZero() {
		last = evt.CreationTimestamp.Time
	}
	if first.IsZero() {
		first = last
	}

	source := evt.ReportingController
	if strings.TrimSpace(source) == "" {
		source = evt.Source.Component
	}

	return EventInfo{
		Type:               evt.Type,
		Reason:             evt.Reason,
		Message:            evt.Message,
		Namespace:          evt.Namespace,
		InvolvedObjectKind: evt.InvolvedObject.Kind,
		InvolvedObjectName: evt.InvolvedObject.Name,
		Count:              evt.Count,
		FirstTimestamp:     first,
		LastTimestamp:      last,
		Source:             source,
	}
}

func mapServiceInfo(svc *corev1.Service) ServiceInfo {
	if svc == nil {
		return ServiceInfo{}
	}

	ports := make([]ServicePortInfo, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		ports = append(ports, ServicePortInfo{
			Name:       port.Name,
			Port:       port.Port,
			Protocol:   string(port.Protocol),
			TargetPort: port.TargetPort.String(),
		})
	}

	return ServiceInfo{
		UID:       string(svc.UID),
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Type:      string(svc.Spec.Type),
		ClusterIP: svc.Spec.ClusterIP,
		Selector:  cloneStringMap(svc.Spec.Selector),
		Ports:     ports,
	}
}

func mapEndpointSliceInfo(slice *discoveryv1.EndpointSlice) EndpointSliceInfo {
	if slice == nil {
		return EndpointSliceInfo{}
	}

	serviceName := slice.Labels[discoveryv1.LabelServiceName]
	addresses := make([]string, 0, len(slice.Endpoints))
	targets := make([]string, 0, len(slice.Endpoints))
	for _, endpoint := range slice.Endpoints {
		addresses = append(addresses, endpoint.Addresses...)
		if endpoint.TargetRef != nil && strings.EqualFold(endpoint.TargetRef.Kind, "Pod") {
			targets = append(targets, podKey(endpoint.TargetRef.Namespace, endpoint.TargetRef.Name))
		}
	}

	return EndpointSliceInfo{
		UID:         string(slice.UID),
		Name:        slice.Name,
		Namespace:   slice.Namespace,
		ServiceName: serviceName,
		Addresses:   addresses,
		PodTargets:  targets,
	}
}

func mapIngressInfo(ing *networkingv1.Ingress) IngressInfo {
	if ing == nil {
		return IngressInfo{}
	}

	hosts := make([]string, 0, len(ing.Spec.Rules))
	backends := make([]IngressBackendInfo, 0, len(ing.Spec.Rules)+1)
	if ing.Spec.DefaultBackend != nil {
		backends = appendIngressBackend(backends, ing.Spec.DefaultBackend)
	}
	for _, rule := range ing.Spec.Rules {
		if strings.TrimSpace(rule.Host) != "" {
			hosts = append(hosts, rule.Host)
		}
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			backends = appendIngressBackend(backends, &path.Backend)
		}
	}

	return IngressInfo{
		UID:       string(ing.UID),
		Name:      ing.Name,
		Namespace: ing.Namespace,
		Hosts:     hosts,
		Backends:  backends,
	}
}

func appendIngressBackend(backends []IngressBackendInfo, backend *networkingv1.IngressBackend) []IngressBackendInfo {
	if backend == nil || backend.Service == nil {
		return backends
	}
	return append(backends, IngressBackendInfo{
		ServiceName: backend.Service.Name,
		ServicePort: ingressServicePortName(backend.Service.Port),
	})
}

func ingressServicePortName(port networkingv1.ServiceBackendPort) string {
	if strings.TrimSpace(port.Name) != "" {
		return port.Name
	}
	if port.Number > 0 {
		return fmt.Sprintf("%d", port.Number)
	}
	return ""
}

func mapQuantities(list corev1.ResourceList) ResourceQuantities {
	out := ResourceQuantities{}
	if cpu := list.Cpu(); cpu != nil {
		out.CPUMilli = cpu.MilliValue()
	}
	if memory := list.Memory(); memory != nil {
		out.MemoryBytes = memory.Value()
	}
	return out
}

func addQuantities(a, b ResourceQuantities) ResourceQuantities {
	return ResourceQuantities{
		CPUMilli:    a.CPUMilli + b.CPUMilli,
		MemoryBytes: a.MemoryBytes + b.MemoryBytes,
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}

func detectNodeRoles(labels map[string]string) []string {
	if len(labels) == 0 {
		return nil
	}

	roles := make([]string, 0, 2)
	for key := range labels {
		if strings.HasPrefix(key, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
			if role == "" {
				role = "worker"
			}
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		if role, ok := labels["kubernetes.io/role"]; ok && strings.TrimSpace(role) != "" {
			roles = append(roles, role)
		}
	}
	return roles
}
