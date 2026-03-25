package main

import (
	"log"
	"net"
	"slices"
	"strings"
)

var ignoredInterfaceKeywords = []string{
	"bluetooth",
	"bridge",
	"docker",
	"hyper-v",
	"loopback",
	"tailscale",
	"tun",
	"virtual",
	"vmware",
	"vbox",
	"wsl",
}

var preferredWiFiKeywords = []string{
	"wi-fi",
	"wifi",
	"wlan",
	"wireless",
}

func isPrivateIPv4(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}

	switch {
	case ip[0] == 10:
		return true
	case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return true
	case ip[0] == 192 && ip[1] == 168:
		return true
	default:
		return false
	}
}

func shouldIgnoreInterface(name string) bool {
	lower := strings.ToLower(name)
	for _, keyword := range ignoredInterfaceKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func isPreferredWiFiInterface(name string) bool {
	lower := strings.ToLower(name)
	for _, keyword := range preferredWiFiKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func GetAllLocalIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Println("[IP] Error getting interfaces:", err)
		return []string{"127.0.0.1"}
	}

	seen := map[string]struct{}{}
	var wifiIPs []string
	var otherIPs []string

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if shouldIgnoreInterface(iface.Name) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || !isPrivateIPv4(ipNet.IP) {
				continue
			}

			ip := ipNet.IP.String()
			if _, exists := seen[ip]; exists {
				continue
			}

			seen[ip] = struct{}{}
			if isPreferredWiFiInterface(iface.Name) {
				wifiIPs = append(wifiIPs, ip)
			} else {
				otherIPs = append(otherIPs, ip)
			}
			log.Printf("[IP] Found LAN IPv4 %s on %s", ip, iface.Name)
		}
	}

	slices.Sort(wifiIPs)
	slices.Sort(otherIPs)

	if len(wifiIPs) > 0 {
		return wifiIPs
	}
	if len(otherIPs) == 0 {
		log.Println("[IP] No LAN IPv4 detected, fallback to 127.0.0.1")
		return []string{"127.0.0.1"}
	}
	return otherIPs
}

func GetLocalIP() string {
	ips := GetAllLocalIPs()
	return ips[0]
}
