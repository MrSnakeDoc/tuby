package utils

import (
	"net"
	"net/http"
	"strings"
)

// ParseHostNoPort returns the host part (no port) from strings like "ip:port", "[v6]:port", or "ip".
func ParseHostNoPort(s string) string {
	if s == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		return h
	}
	return s
}

// FirstForwardedFor returns the first IP from X-Forwarded-For (left-most), trimmed.
func FirstForwardedFor(xff string) string {
	xff = strings.TrimSpace(xff)
	if xff == "" {
		return ""
	}
	if i := strings.IndexByte(xff, ','); i >= 0 {
		xff = xff[:i]
	}
	return strings.TrimSpace(xff)
}

// ClientIP resolves the real client IP.
// If trustProxy is true, prefers CF-Connecting-IP, X-Forwarded-For (first), then X-Real-IP.
// Otherwise falls back to RemoteAddr only.
//
// NOTE: Use trustProxy=true when your origin is only reachable via a trusted reverse proxy/tunnel (e.g., cloudflared on localhost).
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if v := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); v != "" {
			if ip := ParseHostNoPort(v); ip != "" {
				return ip
			}
		}
		if v := FirstForwardedFor(r.Header.Get("X-Forwarded-For")); v != "" {
			if ip := ParseHostNoPort(v); ip != "" {
				return ip
			}
		}
		if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
			if ip := ParseHostNoPort(v); ip != "" {
				return ip
			}
		}
	}
	return ParseHostNoPort(r.RemoteAddr)
}

// IPMatcher matches exact IPs and CIDRs.
type IPMatcher struct {
	ips  []net.IP
	nets []*net.IPNet
}

func NewIPMatcher(list []string) *IPMatcher {
	m := &IPMatcher{}
	for _, raw := range list {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if _, ipnet, err := net.ParseCIDR(s); err == nil {
			m.nets = append(m.nets, ipnet)
			continue
		}
		if ip := net.ParseIP(s); ip != nil {
			m.ips = append(m.ips, ip)
		}
	}
	return m
}

func (m *IPMatcher) IsEmpty() bool {
	return len(m.ips) == 0 && len(m.nets) == 0
}

func (m *IPMatcher) Allow(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, v := range m.ips {
		if v.Equal(ip) {
			return true
		}
	}
	for _, n := range m.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
