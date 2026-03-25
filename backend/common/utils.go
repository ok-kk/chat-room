package common

import (
	"net"
	"strings"
)

// isVirtualIP 判断是否为虚拟网卡IP
func isVirtualIP(ip string) bool {
	// 常见虚拟网卡IP段
	virtualPrefixes := []string{
		"192.168.88.",  // VMware
		"192.168.56.",  // VirtualBox
		"192.168.26.",  // Hyper-V / WSL
		"192.168.208.", // WSL
		"192.168.111.", // VMware
		"172.17.",      // Docker
		"172.18.",
		"172.19.",
		"172.20.",
	}
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(ip, prefix) {
			return true
		}
	}
	return false
}

// GetLocalIP 获取本机局域网IP（排除虚拟网卡）
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	var candidates []string

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
					if !isVirtualIP(ip) {
						candidates = append(candidates, ip)
					}
				}
			}
		}
	}

	// 优先返回 192.168.x.x
	for _, ip := range candidates {
		if strings.HasPrefix(ip, "192.168.") {
			return ip
		}
	}
	// 其次 10.x.x.x
	for _, ip := range candidates {
		if strings.HasPrefix(ip, "10.") {
			return ip
		}
	}
	// 最后 172.x.x.x
	if len(candidates) > 0 {
		return candidates[0]
	}

	return "127.0.0.1"
}

// GetAllLocalIPs 获取所有可用的局域网IP
func GetAllLocalIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{"127.0.0.1"}
	}

	var ips []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
					ips = append(ips, ip)
				}
			}
		}
	}
	if len(ips) == 0 {
		return []string{"127.0.0.1"}
	}
	return ips
}