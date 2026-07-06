package middleware

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const defaultTrustedProxyCIDRs = "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"

// ClientIPResolver resolves the client IP for rate limiting and logging.
// Forwarded headers are only trusted when the immediate TCP peer is a trusted proxy.
type ClientIPResolver struct {
	trustedNets []*net.IPNet
}

func NewClientIPResolver(trustedCIDRs string) (*ClientIPResolver, error) {
	if strings.TrimSpace(trustedCIDRs) == "" {
		trustedCIDRs = defaultTrustedProxyCIDRs
	}

	var nets []*net.IPNet
	for _, cidr := range strings.Split(trustedCIDRs, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		nets = append(nets, network)
	}
	if len(nets) == 0 {
		return nil, &net.ParseError{Type: "CIDR", Text: trustedCIDRs}
	}
	return &ClientIPResolver{trustedNets: nets}, nil
}

func (r *ClientIPResolver) ClientIP(c *fiber.Ctx) string {
	return r.Resolve(c.IP(), c.Get("X-Forwarded-For"), c.Get("X-Real-IP"))
}

func (r *ClientIPResolver) Resolve(peerIP, xForwardedFor, xRealIP string) string {
	peer := net.ParseIP(strings.TrimSpace(peerIP))
	if peer == nil || !r.isTrusted(peer) {
		if peer != nil {
			return peer.String()
		}
		return strings.TrimSpace(peerIP)
	}

	if xForwardedFor != "" {
		client := strings.TrimSpace(strings.Split(xForwardedFor, ",")[0])
		if ip := net.ParseIP(client); ip != nil {
			return ip.String()
		}
	}
	if xRealIP = strings.TrimSpace(xRealIP); xRealIP != "" {
		if ip := net.ParseIP(xRealIP); ip != nil {
			return ip.String()
		}
	}
	return peer.String()
}

func (r *ClientIPResolver) isTrusted(ip net.IP) bool {
	for _, network := range r.trustedNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
