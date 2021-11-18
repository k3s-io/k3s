package utils

import (
	"strings"

	v1core "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// ServiceForEndpoints given Endpoint object return Service API object if it exists
func ServiceForEndpoints(ci *cache.Indexer, ep *v1core.Endpoints) (interface{}, bool, error) {
	key, err := cache.MetaNamespaceKeyFunc(ep)
	if err != nil {
		return nil, false, err
	}

	item, exists, err := (*ci).GetByKey(key)
	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	return item, true, nil
}

// ServiceIsHeadless decides whether or not the this service is a headless service which is often useful to kube-router
// as there is no need to execute logic on most headless changes. Function takes a generic interface as its input
// parameter so that it can be used more easily in early processing if needed. If a non-service object is given,
// function will return false.
func ServiceIsHeadless(obj interface{}) bool {
	if svc, _ := obj.(*v1core.Service); svc != nil {
		if svc.Spec.Type == v1core.ServiceTypeClusterIP {
			if ClusterIPIsNone(svc.Spec.ClusterIP) && containsOnlyNone(svc.Spec.ClusterIPs) {
				return true
			}
		}
	}
	return false
}

// ClusterIPIsNone checks to see whether the ClusterIP contains "None" which would indicate that it is headless
func ClusterIPIsNone(clusterIP string) bool {
	return strings.ToLower(clusterIP) == "none"
}

func ClusterIPIsNoneOrBlank(clusterIP string) bool {
	return ClusterIPIsNone(clusterIP) || clusterIP == ""
}

func containsOnlyNone(clusterIPs []string) bool {
	for _, clusterIP := range clusterIPs {
		if !ClusterIPIsNone(clusterIP) {
			return false
		}
	}
	return true
}
