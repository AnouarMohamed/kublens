package state

import (
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func (c *ClusterCache) onServiceAdd(obj any) {
	svc, ok := obj.(*corev1.Service)
	if !ok || svc == nil {
		return
	}

	info := mapServiceInfo(svc)
	key := podKey(info.Namespace, info.Name)

	c.mu.Lock()
	c.state.Services[key] = info
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("service_update", map[string]any{"namespace": info.Namespace, "service": info.Name})
}

func (c *ClusterCache) onServiceUpdate(oldObj, newObj any) {
	c.onServiceAdd(newObj)
}

func (c *ClusterCache) onServiceDelete(obj any) {
	svc, ok := obj.(*corev1.Service)
	if !ok || svc == nil {
		return
	}

	key := podKey(svc.Namespace, svc.Name)
	c.mu.Lock()
	delete(c.state.Services, key)
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("service_deleted", map[string]any{"namespace": svc.Namespace, "service": svc.Name})
}

func (c *ClusterCache) onEndpointSliceAdd(obj any) {
	slice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok || slice == nil {
		return
	}

	info := mapEndpointSliceInfo(slice)
	key := podKey(info.Namespace, info.Name)

	c.mu.Lock()
	c.state.EndpointSlices[key] = info
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("endpoints_update", map[string]any{"namespace": info.Namespace, "service": info.ServiceName})
}

func (c *ClusterCache) onEndpointSliceUpdate(oldObj, newObj any) {
	c.onEndpointSliceAdd(newObj)
}

func (c *ClusterCache) onEndpointSliceDelete(obj any) {
	slice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok || slice == nil {
		return
	}

	key := podKey(slice.Namespace, slice.Name)
	serviceName := slice.Labels[discoveryv1.LabelServiceName]
	c.mu.Lock()
	delete(c.state.EndpointSlices, key)
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("endpoints_deleted", map[string]any{"namespace": slice.Namespace, "service": serviceName})
}

func (c *ClusterCache) onIngressAdd(obj any) {
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok || ing == nil {
		return
	}

	info := mapIngressInfo(ing)
	key := podKey(info.Namespace, info.Name)

	c.mu.Lock()
	c.state.Ingresses[key] = info
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("ingress_update", map[string]any{"namespace": info.Namespace, "ingress": info.Name})
}

func (c *ClusterCache) onIngressUpdate(oldObj, newObj any) {
	c.onIngressAdd(newObj)
}

func (c *ClusterCache) onIngressDelete(obj any) {
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok || ing == nil {
		return
	}

	key := podKey(ing.Namespace, ing.Name)
	c.mu.Lock()
	delete(c.state.Ingresses, key)
	c.setLastUpdated()
	c.mu.Unlock()

	c.publish("ingress_deleted", map[string]any{"namespace": ing.Namespace, "ingress": ing.Name})
}
