package device

import "net/netip"

// inet_ntoa came from https://go.dev/play/p/JlYJXZnUxl
func inet_ntoa(ipInt64 uint32) (ip netip.Addr) {
	ipArray := [4]byte{byte(ipInt64 >> 24), byte(ipInt64 >> 16), byte(ipInt64 >> 8), byte(ipInt64)}
	ip = netip.AddrFrom4(ipArray)
	return
}

func CountPrograms(channels []ChannelScan) int {
	count := 0
	for i := 0; i < len(channels); i++ {
		count += len(channels[i].Programs)
	}
	return count
}
