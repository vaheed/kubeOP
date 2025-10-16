package dns

import "net/netip"

func splitAddrs(addrs []netip.Addr) ([]netip.Addr, []netip.Addr) {
	ipv4 := make([]netip.Addr, 0, len(addrs))
	ipv6 := make([]netip.Addr, 0, len(addrs))
	for _, addr := range addrs {
		if !addr.IsValid() {
			continue
		}
		if addr.Is6() {
			ipv6 = append(ipv6, addr)
		} else {
			ipv4 = append(ipv4, addr)
		}
	}
	return ipv4, ipv6
}

func normalizeTTL(ttl int) int {
	if ttl <= 0 {
		return 300
	}
	return ttl
}
