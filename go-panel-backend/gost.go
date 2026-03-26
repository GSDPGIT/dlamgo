package main

import (
	"fmt"
	"strings"
)

func limiterPayload(name int64, speedMB string) map[string]interface{} {
	return map[string]interface{}{
		"name": fmt.Sprintf("%d", name),
		"limits": []string{
			fmt.Sprintf("$ %sMB %sMB", speedMB, speedMB),
		},
	}
}

func deleteServicesPayload(names ...string) map[string]interface{} {
	return map[string]interface{}{"services": names}
}

func deleteChainPayload(name string) map[string]interface{} {
	return map[string]interface{}{"chain": name}
}

func createMainServices(name string, inPort int, limiter *int64, remoteAddr string, tunnelType int, tunnel Tunnel, strategy, interfaceName string) []map[string]interface{} {
	protocols := []string{"tcp", "udp"}
	services := make([]map[string]interface{}, 0, len(protocols))
	for _, protocol := range protocols {
		service := map[string]interface{}{
			"name": fmt.Sprintf("%s_%s", name, protocol),
			"addr": fmt.Sprintf("%s:%d", listenerAddress(protocol, tunnel), inPort),
			"handler": map[string]interface{}{
				"type": protocol,
			},
			"listener": listenerPayload(protocol),
		}
		if limiter != nil {
			service["limiter"] = fmt.Sprintf("%d", *limiter)
		}
		if strings.TrimSpace(interfaceName) != "" {
			service["metadata"] = map[string]interface{}{"interface": interfaceName}
		}
		if tunnelType == 2 {
			service["handler"] = map[string]interface{}{
				"type":  protocol,
				"chain": fmt.Sprintf("%s_chains", name),
			}
		} else {
			service["forwarder"] = forwarderPayload(remoteAddr, strategy)
		}
		services = append(services, service)
	}
	return services
}

func createRemoteService(name string, outPort int, remoteAddr, protocol, strategy, interfaceName string) []map[string]interface{} {
	service := map[string]interface{}{
		"name": fmt.Sprintf("%s_tls", name),
		"addr": fmt.Sprintf(":%d", outPort),
		"handler": map[string]interface{}{
			"type": "relay",
		},
		"listener": map[string]interface{}{
			"type": protocol,
		},
		"forwarder": forwarderPayload(remoteAddr, strategy),
	}
	if strings.TrimSpace(interfaceName) != "" {
		service["metadata"] = map[string]interface{}{"interface": interfaceName}
	}
	return []map[string]interface{}{service}
}

func createChain(name, remoteAddr, protocol, interfaceName string) map[string]interface{} {
	dialer := map[string]interface{}{"type": protocol}
	if protocol == "quic" {
		dialer["metadata"] = map[string]interface{}{
			"keepAlive": true,
			"ttl":       "10s",
		}
	}
	node := map[string]interface{}{
		"name":      "node-" + name,
		"addr":      remoteAddr,
		"connector": map[string]interface{}{"type": "relay"},
		"dialer":    dialer,
	}
	if strings.TrimSpace(interfaceName) != "" {
		node["interface"] = interfaceName
	}
	return map[string]interface{}{
		"name": fmt.Sprintf("%s_chains", name),
		"hops": []map[string]interface{}{
			{
				"name":  "hop-" + name,
				"nodes": []map[string]interface{}{node},
			},
		},
	}
}

func listenerPayload(protocol string) map[string]interface{} {
	if protocol == "udp" {
		return map[string]interface{}{
			"type": protocol,
			"metadata": map[string]interface{}{
				"keepAlive": true,
			},
		}
	}
	return map[string]interface{}{"type": protocol}
}

func listenerAddress(protocol string, tunnel Tunnel) string {
	if protocol == "udp" {
		return tunnel.UDPListenAddr
	}
	return tunnel.TCPListenAddr
}

func forwarderPayload(remoteAddr, strategy string) map[string]interface{} {
	addresses := splitAddressList(remoteAddr)
	nodes := make([]map[string]interface{}, 0, len(addresses))
	for index, address := range addresses {
		nodes = append(nodes, map[string]interface{}{
			"name": fmt.Sprintf("node_%d", index+1),
			"addr": address,
		})
	}
	if strings.TrimSpace(strategy) == "" {
		strategy = "fifo"
	}
	return map[string]interface{}{
		"nodes": nodes,
		"selector": map[string]interface{}{
			"strategy":    strategy,
			"maxFails":    1,
			"failTimeout": "600s",
		},
	}
}

func splitAddressList(raw string) []string {
	parts := strings.Split(raw, ",")
	addresses := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			addresses = append(addresses, part)
		}
	}
	return addresses
}
