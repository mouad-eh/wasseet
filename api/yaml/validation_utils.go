package yaml

import (
	"fmt"
	"net"
	"regexp"
)

func isValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

var dnsRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)

func isValidDNSOrIPWithPort(addr string) bool {
	if addr == "" {
		return false
	}
	// Try to split host and port
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		// Has port: must be IP:port format or hostname:port format
		if net.ParseIP(host) == nil && !dnsRegex.MatchString(host) {
			return false
		}
		// Validate port is a valid number
		portNum := 0
		_, err = fmt.Sscanf(port, "%d", &portNum)
		return err == nil && isValidPort(portNum)
	}
	// No port: must be hostname only
	return dnsRegex.MatchString(addr)
}

func isValidLoadBalancingType(lbt LoadBalancingType) bool {
	return lbt == "" || validLoadBalancingTypes[lbt]
}
