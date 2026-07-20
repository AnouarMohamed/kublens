package cluster

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	"kubelens-backend/internal/model"
)

func mapContainerSpec(container corev1.Container) model.ContainerSpec {
	out := model.ContainerSpec{
		Name:         container.Name,
		Image:        container.Image,
		Env:          make([]model.ContainerEnv, 0, len(container.Env)),
		VolumeMounts: make([]model.VolumeMount, 0, len(container.VolumeMounts)),
	}

	for _, env := range container.Env {
		out.Env = append(out.Env, model.ContainerEnv{Name: env.Name, Value: env.Value})
	}

	for _, vm := range container.VolumeMounts {
		out.VolumeMounts = append(out.VolumeMounts, model.VolumeMount{
			Name:      vm.Name,
			MountPath: vm.MountPath,
		})
	}

	if len(container.Resources.Requests) > 0 || len(container.Resources.Limits) > 0 {
		resources := &model.ContainerResources{}
		if len(container.Resources.Requests) > 0 {
			resources.Requests = &model.ResourcePairs{
				CPU:    container.Resources.Requests.Cpu().String(),
				Memory: container.Resources.Requests.Memory().String(),
			}
		}
		if len(container.Resources.Limits) > 0 {
			resources.Limits = &model.ResourcePairs{
				CPU:    container.Resources.Limits.Cpu().String(),
				Memory: container.Resources.Limits.Memory().String(),
			}
		}
		out.Resources = resources
	}

	return out
}

func mapPodSummary(pod corev1.Pod) model.PodSummary {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}

	id := string(pod.UID)
	if id == "" {
		id = pod.Namespace + "-" + pod.Name
	}

	return model.PodSummary{
		ID:        id,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		NodeName:  pod.Spec.NodeName,
		Status:    mapPodStatus(pod.Status.Phase),
		CPU:       "N/A",
		Memory:    "N/A",
		Age:       formatAge(pod.CreationTimestamp.Time),
		Restarts:  restarts,
	}
}

func mapNodeSummary(node corev1.Node) model.NodeSummary {
	return model.NodeSummary{
		Name:          node.Name,
		Status:        mapNodeStatus(node.Status.Conditions),
		Roles:         nodeRoles(node.Labels),
		Unschedulable: node.Spec.Unschedulable,
		Age:           formatAge(node.CreationTimestamp.Time),
		Version:       node.Status.NodeInfo.KubeletVersion,
		CPUUsage:      "N/A",
		MemUsage:      "N/A",
		CPUHistory:    buildCPUHistory(node.Name),
	}
}

func mapPodStatus(phase corev1.PodPhase) model.PodStatus {
	switch phase {
	case corev1.PodRunning:
		return model.PodStatusRunning
	case corev1.PodPending:
		return model.PodStatusPending
	case corev1.PodFailed:
		return model.PodStatusFailed
	case corev1.PodSucceeded:
		return model.PodStatusSucceeded
	default:
		return model.PodStatusUnknown
	}
}

func mapNodeStatus(conditions []corev1.NodeCondition) model.NodeStatus {
	for _, condition := range conditions {
		if condition.Type != corev1.NodeReady {
			continue
		}
		if condition.Status == corev1.ConditionTrue {
			return model.NodeStatusReady
		}
		return model.NodeStatusNotReady
	}
	return model.NodeStatusUnknown
}

func nodeRoles(labels map[string]string) string {
	if len(labels) == 0 {
		return "worker"
	}

	roles := make([]string, 0, 2)
	for key := range labels {
		if !strings.HasPrefix(key, "node-role.kubernetes.io/") {
			continue
		}
		if role := strings.TrimPrefix(key, "node-role.kubernetes.io/"); role != "" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "worker"
	}
	return strings.Join(roles, ",")
}
