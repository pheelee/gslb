package health

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ICMPChecker performs ICMP ping health checks.
//
// It first tries unprivileged ICMP (udp4 / SOCK_DGRAM), which works on Linux
// when the process GID is within /proc/sys/net/ipv4/ping_group_range (default
// on most distributions: 0–2147483647).  If that fails it falls back to a raw
// socket (ip4:icmp / SOCK_RAW), which requires root or CAP_NET_RAW.
type ICMPChecker struct {
	target  string
	timeout time.Duration
}

// NewICMPChecker creates a new ICMP checker for the given target.
func NewICMPChecker(target string, timeout time.Duration) *ICMPChecker {
	return &ICMPChecker{target: target, timeout: timeout}
}

// Target returns the target IP address.
func (c *ICMPChecker) Target() string {
	return c.target
}

// Check performs an ICMP echo request (ping) to the target.
func (c *ICMPChecker) Check(ctx context.Context) (bool, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Strip port if present (ICMP works on IPs only).
	host := c.target
	if h, _, err := net.SplitHostPort(c.target); err == nil {
		host = h
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Resolve hostname
		addrs, err := net.LookupHost(host)
		if err != nil || len(addrs) == 0 {
			return false, fmt.Errorf("failed to resolve %s: %w", host, err)
		}
		ip = net.ParseIP(addrs[0])
	}
	ip = ip.To4()
	if ip == nil {
		return false, fmt.Errorf("only IPv4 targets are supported, got %s", host)
	}

	// Try unprivileged ICMP first (udp4/SOCK_DGRAM); fall back to raw socket.
	conn, network, err := openICMPConn()
	if err != nil {
		return false, fmt.Errorf("failed to open ICMP socket: %w", err)
	}
	defer conn.Close()

	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return false, fmt.Errorf("set deadline: %w", err)
	}

	// Unique ID per check prevents cross-contamination between concurrent checks.
	id := int(rand.N(uint16(0xfffe))) + 1 // 1–65534
	seq := 1

	msg := &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: id, Seq: seq, Data: []byte("gslb-hc")},
	}
	data, err := msg.Marshal(nil)
	if err != nil {
		return false, fmt.Errorf("marshal ICMP: %w", err)
	}

	dst := writeAddr(network, ip)
	if _, err := conn.WriteTo(data, dst); err != nil {
		return false, fmt.Errorf("send ICMP echo to %s: %w", ip, err)
	}

	// Read loop: raw sockets receive all ICMP; keep reading until we find our
	// reply or the deadline fires.
	buf := make([]byte, 1500)
	for {
		n, peer, err := conn.ReadFrom(buf)
		if err != nil {
			if isNetTimeout(err) {
				return false, nil // timeout → unhealthy, not an error worth logging
			}
			return false, fmt.Errorf("read ICMP: %w", err)
		}

		rcv, parseErr := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), buf[:n])
		if parseErr != nil || rcv.Type != ipv4.ICMPTypeEchoReply {
			continue
		}

		echo, ok := rcv.Body.(*icmp.Echo)
		if !ok {
			continue
		}

		// With udp4 (SOCK_DGRAM) the kernel routes replies by ID automatically,
		// so we only need to verify seq. With ip4:icmp (raw) we must also check
		// ID and peer to filter out unrelated ICMP traffic.
		if network == "udp4" {
			if echo.Seq == seq {
				return true, nil
			}
		} else {
			peerIP := peerToIP(peer)
			if echo.ID == id && echo.Seq == seq && peerIP.Equal(ip) {
				return true, nil
			}
		}
		// Not our reply — keep reading.
	}
}

// openICMPConn tries udp4 (unprivileged) then ip4:icmp (raw/privileged).
func openICMPConn() (*icmp.PacketConn, string, error) {
	if conn, err := icmp.ListenPacket("udp4", ""); err == nil {
		return conn, "udp4", nil
	}
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, "", err
	}
	return conn, "ip4:icmp", nil
}

// writeAddr returns the correct net.Addr for WriteTo depending on network type.
func writeAddr(network string, ip net.IP) net.Addr {
	if network == "udp4" {
		return &net.UDPAddr{IP: ip}
	}
	return &net.IPAddr{IP: ip}
}

// peerToIP extracts the IP from a net.Addr returned by ReadFrom.
func peerToIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.IPAddr:
		return a.IP
	case *net.UDPAddr:
		return a.IP
	}
	host, _, _ := net.SplitHostPort(addr.String())
	return net.ParseIP(host)
}

// isNetTimeout reports whether the error is a network timeout.
func isNetTimeout(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}
