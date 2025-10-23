package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"time"
)

type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

var (
	privateCIDRs = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("fe80::/10"),
	}
)

var (
	ErrInvalidURL      = errors.New("invalid https url")
	ErrForbiddenHost   = errors.New("host not permitted")
	ErrDisallowedIP    = errors.New("ip not permitted")
	ErrRedirectLimited = errors.New("redirect limit exceeded")
)

func AllowedHost(host string, allowlist []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if len(allowlist) == 0 {
		return false
	}
	for _, rule := range allowlist {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		if rule == "*" {
			return true
		}
		if strings.HasPrefix(rule, "*.") {
			suffix := strings.TrimPrefix(rule, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == rule {
			return true
		}
	}
	return false
}

func HasDotDot(path string) bool {
	if strings.Contains(path, "..") {
		return true
	}
	lower := strings.ToLower(path)
	if strings.Contains(lower, "%2e") {
		return true
	}
	return false
}

func hostWithPort(host, port string) string {
	if port == "" || port == "443" {
		return host
	}
	return net.JoinHostPort(host, port)
}

func ParseAndValidateHTTPSURL(raw string, allow func(string) bool) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	return ValidateHTTPSURL(parsed, allow)
}

func ValidateHTTPSURL(u *url.URL, allow func(string) bool) (*url.URL, error) {
	if u == nil {
		return nil, ErrInvalidURL
	}
	if u.Opaque != "" {
		return nil, fmt.Errorf("%w: opaque urls are not allowed", ErrInvalidURL)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return nil, fmt.Errorf("%w: scheme must be https", ErrInvalidURL)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: credentials not permitted", ErrInvalidURL)
	}
	if HasDotDot(u.RawPath) || HasDotDot(u.Path) {
		return nil, fmt.Errorf("%w: path must not contain dot segments", ErrInvalidURL)
	}
	if strings.Contains(u.Path, "\\") {
		return nil, fmt.Errorf("%w: backslashes are not permitted", ErrInvalidURL)
	}
	if decoded, err := url.PathUnescape(u.Path); err != nil {
		return nil, fmt.Errorf("%w: invalid path encoding", ErrInvalidURL)
	} else {
		if decoded == "" || decoded[0] != '/' {
			return nil, fmt.Errorf("%w: path must be absolute", ErrInvalidURL)
		}
		if HasDotDot(decoded) {
			return nil, fmt.Errorf("%w: path must not contain dot segments", ErrInvalidURL)
		}
		for _, seg := range strings.Split(decoded, "/") {
			if seg == "." {
				return nil, fmt.Errorf("%w: path must not contain dot segments", ErrInvalidURL)
			}
		}
	}
	if port := u.Port(); port != "" && port != "443" {
		return nil, fmt.Errorf("%w: port must be 443", ErrInvalidURL)
	}
	if isIPLiteral(host) {
		return nil, fmt.Errorf("%w: ip literals are not allowed", ErrForbiddenHost)
	}
	if strings.EqualFold(host, "localhost") {
		return nil, fmt.Errorf("%w: localhost not allowed", ErrForbiddenHost)
	}
	if allow != nil && !allow(host) {
		return nil, ErrForbiddenHost
	}
	sanitizedPath := path.Clean(strings.ReplaceAll(u.Path, "\\", "/"))
	if sanitizedPath == "." {
		sanitizedPath = "/"
	}
	sanitized := &url.URL{
		Scheme:   "https",
		Host:     hostWithPort(host, u.Port()),
		Path:     sanitizedPath,
		RawQuery: u.Query().Encode(),
	}
	if sanitized.RawQuery == "" {
		sanitized.RawQuery = ""
	}
	return sanitized, nil
}

func pathClean(p string) string {
	if p == "" {
		return "/"
	}
	cleaned := strings.ReplaceAll(p, "\\", "/")
	segments := strings.Split(cleaned, "/")
	var builder strings.Builder
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		builder.WriteByte('/')
		builder.WriteString(seg)
	}
	if builder.Len() == 0 {
		return "/"
	}
	return builder.String()
}

func isIPLiteral(host string) bool {
	if host == "" {
		return false
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.Trim(host, "[]")
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	return false
}

func IsPublicAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range privateCIDRs {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

func DenyPrivateNetworks(base DialContextFunc) DialContextFunc {
	if base == nil {
		dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
		base = dialer.DialContext
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if host == "" {
			return nil, fmt.Errorf("%w: empty dial host", ErrDisallowedIP)
		}
		if ip := net.ParseIP(host); ip != nil {
			addr, ok := netip.AddrFromSlice(ip)
			if !ok {
				return nil, fmt.Errorf("%w: invalid ip %s", ErrDisallowedIP, host)
			}
			if !IsPublicAddr(addr) {
				return nil, fmt.Errorf("%w: %s", ErrDisallowedIP, addr.String())
			}
			return base(ctx, network, address)
		}
		conn, err := base(ctx, network, address)
		if err != nil {
			return nil, err
		}
		remoteHost, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			conn.Close()
			return nil, err
		}
		ip := net.ParseIP(remoteHost)
		if ip == nil {
			conn.Close()
			return nil, fmt.Errorf("%w: non-ip remote %s", ErrDisallowedIP, remoteHost)
		}
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			conn.Close()
			return nil, fmt.Errorf("%w: invalid ip %s", ErrDisallowedIP, remoteHost)
		}
		if !IsPublicAddr(addr) {
			conn.Close()
			return nil, fmt.Errorf("%w: %s", ErrDisallowedIP, addr.String())
		}
		return conn, nil
	}
}

func ValidateRedirect(req *http.Request, via []*http.Request, allow func(string) bool) error {
	if len(via) >= 3 {
		return ErrRedirectLimited
	}
	if req == nil || req.URL == nil {
		return ErrInvalidURL
	}
	sanitized, err := ValidateHTTPSURL(req.URL, allow)
	if err != nil {
		return err
	}
	req.URL = sanitized
	req.Host = sanitized.Host
	return nil
}
