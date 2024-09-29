package utils

import (
	"net"
)

func CurrentIPv4() (*string, error) {
	var currentIP string
	// Get a list of all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip down interfaces and loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Get the addresses associated with the interface
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			// Get the IP address from the address
			ipNet, ok := addr.(*net.IPNet)
			if ok && ipNet.IP != nil {
				// Check if the IP is IPv4 or IPv6 and print it
				if ipNet.IP.To4() != nil {
					currentIP = ipNet.IP.String()
					return &currentIP, nil
				}
			}
		}
	}

	return nil, err
}
