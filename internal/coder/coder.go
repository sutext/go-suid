package coder

import (
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
)

var (
	HOSTID uint8
	ENCODE = "abcdefghijklmnopqrstuvwxyz123456" // custom base32 encoding map
	DECODE = make(map[byte]byte, len(ENCODE))
)

func init() {
	HOSTID = getHostID()
	for i := 0; i < len(ENCODE); i++ {
		DECODE[ENCODE[i]] = byte(i)
	}
}

func getHostID() uint8 {
	// read SUID_HOST_ID from environment firstly
	str := os.Getenv("SUID_HOST_ID")
	if str != "" {
		id, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return uint8(id)
		}
	}
	// read statefulset id from k8s environment secondly
	name := os.Getenv("POD_NAME")
	if name == "" {
		name, _ = os.Hostname()
	}
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		str := parts[len(parts)-1]
		id, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return uint8(id)
		}
	}
	// read local ip address thirdly
	if ip, err := getLocalIP(); err == nil {
		parts := strings.Split(ip, ".")
		if len(parts) == 4 {
			last, err := strconv.ParseInt(parts[3], 10, 64)
			if err == nil {
				return uint8(last)
			}
		}
	}
	return uint8(rand.Int())
}

// getLocalIP retrieves the non-loopback local IPv4 address of the machine.
func getLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		// Check if the interface is up and not a loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no network interface found")
}
