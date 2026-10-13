package config

import (
	"fmt"
	"log"
	"net"
	"strings"
)

func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}

// isPrivateIP reports whether the host is a private (RFC 1918) IP address.
// Allows HTTP for LAN development without requiring TLS certificates.
func isPrivateIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	// Convert to 4-byte IPv4 representation if it's an IPv4-mapped IPv6 address.
	ip4 := ip.To4()
	if ip4 == nil {
		return false // not an IPv4 address
	}
	// 10.0.0.0/8
	if ip4[0] == 10 {
		return true
	}
	// 172.16.0.0/12
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	// 192.168.0.0/16
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	return false
}

// for gin's SetTrustedProxies. Empty input returns nil, which preserves
// today's behavior (ClientIP returns the direct peer, X-Forwarded-For
// ignored). Open ranges 0.0.0.0/0 and ::/0 are rejected because trusting every
// address would let any client spoof X-Forwarded-For and evade per-IP rate
// limits.
func parseTrustedProxies(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == "0.0.0.0/0" || entry == "::/0" {
			return nil, fmt.Errorf("TRUSTED_PROXIES %q trusts every address and would allow IP spoofing; list only your proxy addresses or CIDRs", entry)
		}
		if strings.Contains(entry, "/") {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES %q is not a valid IP or CIDR", entry)
			}
		} else if ip := net.ParseIP(entry); ip == nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES %q is not a valid IP or CIDR", entry)
		}
		out = append(out, entry)
	}
	return out, nil
}

// mustParseTrustedProxies fails fast at startup on an invalid entry.
func mustParseTrustedProxies(raw string) []string {
	parsed, err := parseTrustedProxies(raw)
	if err != nil {
		log.Fatal(err)
	}
	return parsed
}
