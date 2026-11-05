package storage

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// SSRFPolicy configures domain whitelist and network dialer restrictions.
type SSRFPolicy struct {
	AllowedDomains []string
}

// IsDomainAllowed checks whether a hostname satisfies the configured domain whitelist.
func (p *SSRFPolicy) IsDomainAllowed(host string) bool {
	if len(p.AllowedDomains) == 0 {
		return true // If no domain filter configured, permit all public domains
	}

	cleanHost := strings.ToLower(strings.TrimSpace(host))
	// Strip port if present
	if h, _, err := net.SplitHostPort(cleanHost); err == nil {
		cleanHost = h
	}

	for _, pattern := range p.AllowedDomains {
		pat := strings.ToLower(strings.TrimSpace(pattern))
		if pat == cleanHost {
			return true
		}
		if matched, _ := filepath.Match(pat, cleanHost); matched {
			return true
		}
	}
	return false
}

// IsRestrictedIP tests if an IP address belongs to private, loopback, link-local, or cloud metadata ranges.
func IsRestrictedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	// Standard Go checks (covers 127.0.0.1, ::1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	// Explicit AWS/GCP/Azure link-local metadata (169.254.169.254) and 169.254.0.0/16
	ipv4 := ip.To4()
	if ipv4 != nil {
		// 169.254.x.x
		if ipv4[0] == 169 && ipv4[1] == 254 {
			return true
		}
		// 0.x.x.x
		if ipv4[0] == 0 {
			return true
		}
		// 224.x.x.x+ (multicast & reserved)
		if ipv4[0] >= 224 {
			return true
		}
	}

	return false
}

// NewSafeHTTPClient returns an http.Client with a dialer that blocks private IPs and DNS rebinding attacks.
func NewSafeHTTPClient(policy SSRFPolicy, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			return nil
		},
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			// Validate domain whitelist
			if !policy.IsDomainAllowed(host) {
				return nil, fmt.Errorf("%w: domain %q not in whitelist", ErrSSRFBlocked, host)
			}

			// Resolve host to IPs
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no IP addresses resolved for %q", host)
			}

			// Verify every resolved IP to defend against DNS rebinding
			for _, ip := range ips {
				if IsRestrictedIP(ip) {
					return nil, fmt.Errorf("%w: resolved IP %s is restricted", ErrSSRFBlocked, ip.String())
				}
			}

			// Connect to the first verified safe IP
			targetAddr := net.JoinHostPort(ips[0].String(), port)
			return dialer.DialContext(ctx, network, targetAddr)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}
