// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later
// Package netguard keeps connections that a workspace member configures, such
// as a webhook URL or an SMTP host, from reaching the server's own network:
// loopback, private ranges, link-local addresses (cloud metadata endpoints
// among them) and the like.
//
// The check runs on the address actually dialled, after DNS resolution, so a
// hostname that resolves to a private address, a DNS answer that changes
// between saving and sending, and a redirect to an internal URL are all
// caught the same way.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

// ErrBlocked is returned for a destination that is not on the public internet.
var ErrBlocked = errors.New("destination is a private or reserved address")

// Policy decides which addresses outbound connections may reach.
type Policy struct {
	// AllowPrivate turns the guard off: any address may be reached.
	AllowPrivate bool
	// Allowed are networks reachable even though they are not public, such
	// as the subnet of an internal automation service.
	Allowed []netip.Prefix
}

// nonPublic are the ranges a connection must not reach unless allowed. The
// unspecified address and the other special-purpose ranges are blocked too:
// 0.0.0.0 reaches the local host on most systems.
var nonPublic = mustPrefixes(
	"0.0.0.0/8",       // "this network"; 0.0.0.0 is the local host
	"10.0.0.0/8",      // private
	"100.64.0.0/10",   // carrier-grade NAT
	"127.0.0.0/8",     // loopback
	"169.254.0.0/16",  // link-local, cloud metadata
	"172.16.0.0/12",   // private
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // documentation
	"192.168.0.0/16",  // private
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // documentation
	"203.0.113.0/24",  // documentation
	"224.0.0.0/4",     // multicast
	"240.0.0.0/4",     // reserved, broadcast
	"::/128",          // unspecified
	"::1/128",         // loopback
	"64:ff9b:1::/48",  // local-use NAT64
	"100::/64",        // discard
	"2001:db8::/32",   // documentation
	"fc00::/7",        // unique local
	"fe80::/10",       // link-local
	"ff00::/8",        // multicast
)

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(cidrs))
	for i, c := range cidrs {
		out[i] = netip.MustParsePrefix(c)
	}
	return out
}

// Check reports whether ip may be reached.
func (p Policy) Check(ip netip.Addr) error {
	if p.AllowPrivate {
		return nil
	}
	// An IPv4 address written as IPv6 (::ffff:10.0.0.1) is judged as IPv4.
	ip = ip.Unmap()
	for _, allowed := range p.Allowed {
		if allowed.Contains(ip) {
			return nil
		}
	}
	for _, blocked := range nonPublic {
		if blocked.Contains(ip) {
			return ErrBlocked
		}
	}
	return nil
}

// control is a net.Dialer Control function: it runs once the address has been
// resolved, just before connecting, for every address the dialer tries.
func (p Policy) control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return ErrBlocked
	}
	return p.Check(ap.Addr())
}

// Dialer returns a dialer that refuses addresses the policy blocks.
func (p Policy) Dialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second, Control: p.control}
}

// DialContext is the policy's dialer as an http.Transport DialContext.
func (p Policy) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return p.Dialer(30*time.Second).DialContext(ctx, network, addr)
}

// CheckURL checks a URL before it is saved: http or https, a host, and a host
// that does not resolve to a blocked address. It gives a clear error early;
// the dialer still decides at send time, when DNS may answer differently.
func (p Policy) CheckURL(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("the URL must start with http:// or https://")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("the URL has no host")
	}
	if u.User != nil {
		return fmt.Errorf("the URL must not contain credentials")
	}
	return p.CheckHost(ctx, host)
}

// CheckHost resolves host and refuses it when any of its addresses is
// blocked. A host that does not resolve is accepted: it may exist later, and
// the dialer checks again then.
func (p Policy) CheckHost(ctx context.Context, host string) error {
	if p.AllowPrivate {
		return nil
	}
	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		if p.Check(ip) != nil {
			return fmt.Errorf("%q is a private or reserved address", host)
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil
	}
	for _, ip := range addrs {
		if p.Check(ip) != nil {
			return fmt.Errorf("%q resolves to a private or reserved address", host)
		}
	}
	return nil
}

// FromEnv reads a policy from <prefix>_ALLOW_PRIVATE_NETWORKS (a boolean) and
// <prefix>_ALLOWED_NETWORKS (comma-separated CIDRs or single addresses).
// Entries that do not parse are returned as an error alongside the rest.
func FromEnv(prefix string, defaultAllowPrivate bool) (Policy, error) {
	p := Policy{AllowPrivate: defaultAllowPrivate}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(prefix + "_ALLOW_PRIVATE_NETWORKS"))) {
	case "1", "true", "yes", "on":
		p.AllowPrivate = true
	case "0", "false", "no", "off":
		p.AllowPrivate = false
	}
	var bad []string
	for _, entry := range strings.Split(os.Getenv(prefix+"_ALLOWED_NETWORKS"), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if prefixVal, err := netip.ParsePrefix(entry); err == nil {
			p.Allowed = append(p.Allowed, prefixVal.Masked())
			continue
		}
		if ip, err := netip.ParseAddr(entry); err == nil {
			p.Allowed = append(p.Allowed, netip.PrefixFrom(ip.Unmap(), ip.Unmap().BitLen()))
			continue
		}
		bad = append(bad, entry)
	}
	if len(bad) > 0 {
		return p, fmt.Errorf("%s_ALLOWED_NETWORKS: not a network or address: %s", prefix, strings.Join(bad, ", "))
	}
	return p, nil
}

// Describe turns a connection error into a short reason fit to show a
// workspace member: what went wrong, without the addresses and ports a raw
// network error carries, which would map the server's network.
func Describe(err error) string {
	if err == nil {
		return ""
	}
	var dnsErr *net.DNSError
	var netErr net.Error
	msg := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, ErrBlocked):
		return "blocked: the destination is a private or reserved address"
	case errors.As(err, &dnsErr):
		return "the host name could not be resolved"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "the connection timed out"
	case errors.Is(err, syscall.ECONNREFUSED), strings.Contains(msg, "connection refused"):
		return "the connection was refused"
	case errors.Is(err, syscall.ECONNRESET), strings.Contains(msg, "connection reset"), errors.Is(err, net.ErrClosed), strings.Contains(msg, "eof"):
		return "the connection was closed unexpectedly"
	case strings.Contains(msg, "certificate"), strings.Contains(msg, "tls"), strings.Contains(msg, "x509"):
		return "the TLS handshake failed"
	case strings.Contains(msg, "no route"), strings.Contains(msg, "unreachable"):
		return "the destination is unreachable"
	}
	return "the connection failed"
}
