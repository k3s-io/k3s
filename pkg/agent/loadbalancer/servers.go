package loadbalancer

import (
	"errors"
	"math/rand"
	"reflect"
)

func (lb *LoadBalancer) setServers(serverAddresses []string) bool {
	serverAddresses, hasOriginalServer := sortServers(serverAddresses, lb.originalServerAddress)
	if len(serverAddresses) == 0 {
		return false
	}

	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	if reflect.DeepEqual(serverAddresses, lb.ServerAddresses) {
		return false
	}

	lb.ServerAddresses = serverAddresses
	lb.randomServers = append([]string{}, lb.ServerAddresses...)
	rand.Shuffle(len(lb.randomServers), func(i, j int) {
		lb.randomServers[i], lb.randomServers[j] = lb.randomServers[j], lb.randomServers[i]
	})
	if !hasOriginalServer {
		lb.randomServers = append(lb.randomServers, lb.originalServerAddress)
	}
	lb.currentServerAddress = lb.randomServers[0]
	lb.nextServerIndex = 1

	return true
}

func (lb *LoadBalancer) nextServer(failedServer string) (string, error) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	if len(lb.randomServers) == 0 {
		return "", errors.New("No servers in load balancer proxy list")
	}
	if len(lb.randomServers) == 1 {
		return lb.currentServerAddress, nil
	}
	if failedServer != lb.currentServerAddress {
		return lb.currentServerAddress, nil
	}
	if lb.nextServerIndex >= len(lb.randomServers) {
		lb.nextServerIndex = 0
	}

	lb.currentServerAddress = lb.randomServers[lb.nextServerIndex]
	lb.nextServerIndex++

	return lb.currentServerAddress, nil
}
