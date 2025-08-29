package outbound

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"

	"github.com/metacubex/mihomo/component/resolver"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/transport/socks5"
)

func serializesSocksAddr(metadata *C.Metadata) []byte {
	var buf [][]byte
	addrType := metadata.AddrType()
	p := uint(metadata.DstPort)
	port := []byte{uint8(p >> 8), uint8(p & 0xff)}
	switch addrType {
	case C.AtypDomainName:
		lenM := uint8(len(metadata.Host))
		host := []byte(metadata.Host)
		buf = [][]byte{{socks5.AtypDomainName, lenM}, host, port}
	case C.AtypIPv4:
		host := metadata.DstIP.AsSlice()
		buf = [][]byte{{socks5.AtypIPv4}, host, port}
	case C.AtypIPv6:
		host := metadata.DstIP.AsSlice()
		buf = [][]byte{{socks5.AtypIPv6}, host, port}
	}
	return bytes.Join(buf, nil)
}

func resolveUDPAddr(ctx context.Context, network, address string, prefer C.DNSPrefer) (*net.UDPAddr, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var ip netip.Addr
	var port2 uint16
	switch prefer {
	case C.IPv4Only:
		ip, port2, err = resolver.ResolveIPv4WithResolver(ctx, host, resolver.ProxyServerHostResolver)
	case C.IPv6Only:
		ip, port2, err = resolver.ResolveIPv6WithResolver(ctx, host, resolver.ProxyServerHostResolver)
	case C.IPv6Prefer:
		ip, port2, err = resolver.ResolveIPPrefer6WithResolver(ctx, host, resolver.ProxyServerHostResolver)
	default:
		ip, port2, err = resolver.ResolveIPWithResolver(ctx, host, resolver.ProxyServerHostResolver)
	}

	if err != nil {
		return nil, err
	}
	if port2 != 0 {
		port = strconv.Itoa(int(port2))
	}
	ip, port = resolver.LookupIP4P(ip, port)
	return net.ResolveUDPAddr(network, net.JoinHostPort(ip.String(), port))
}

func safeConnClose(c net.Conn, err error) {
	if err != nil && c != nil {
		_ = c.Close()
	}
}

var rateStringRegexp = regexp.MustCompile(`^(\d+)\s*([KMGT]?)([Bb])ps$`)

func StringToBps(s string) uint64 {
	if s == "" {
		return 0
	}

	// when have not unit, use Mbps
	if v, err := strconv.Atoi(s); err == nil {
		return StringToBps(fmt.Sprintf("%d Mbps", v))
	}

	m := rateStringRegexp.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	var n uint64 = 1
	switch m[2] {
	case "T":
		n *= 1000
		fallthrough
	case "G":
		n *= 1000
		fallthrough
	case "M":
		n *= 1000
		fallthrough
	case "K":
		n *= 1000
	}
	v, _ := strconv.ParseUint(m[1], 10, 64)
	n *= v
	if m[3] == "b" {
		// Bits, need to convert to bytes
		n /= 8
	}
	return n
}
