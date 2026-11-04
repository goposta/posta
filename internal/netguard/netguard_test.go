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
package netguard

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.8.9.10", "10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255",
		"::1", "::", "fe80::1", "fd00::1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254",
	}
	public := []string{"8.8.8.8", "1.1.1.1", "172.32.0.1", "100.128.0.1", "2001:4860:4860::8888", "::ffff:8.8.8.8"}

	var p Policy
	for _, s := range blocked {
		if p.Check(netip.MustParseAddr(s)) == nil {
			t.Errorf("%s: not blocked", s)
		}
	}
	for _, s := range public {
		if err := p.Check(netip.MustParseAddr(s)); err != nil {
			t.Errorf("%s: blocked: %v", s, err)
		}
	}

	if (Policy{AllowPrivate: true}).Check(netip.MustParseAddr("127.0.0.1")) != nil {
		t.Error("AllowPrivate still blocks loopback")
	}
	allowed := Policy{Allowed: []netip.Prefix{netip.MustParsePrefix("10.0.5.0/24")}}
	if allowed.Check(netip.MustParseAddr("10.0.5.7")) != nil {
		t.Error("an allowed network is blocked")
	}
	if allowed.Check(netip.MustParseAddr("10.0.6.7")) == nil {
		t.Error("allowing one network allowed another")
	}
}

func TestCheckURL(t *testing.T) {
	ctx := context.Background()
	var p Policy
	for _, u := range []string{
		"ftp://example.com/hook", "file:///etc/passwd", "gopher://example.com", "http://", "not a url",
		"http://user:pass@example.com/", "http://127.0.0.1:8080/", "http://[::1]/", "http://169.254.169.254/",
		"http://localhost:9000/",
	} {
		if p.CheckURL(ctx, u) == nil {
			t.Errorf("%s: accepted", u)
		}
	}
	if err := p.CheckURL(ctx, "https://203.0.114.1/hook"); err != nil {
		t.Errorf("public address refused: %v", err)
	}
	// A name that does not resolve is left to the dialer.
	if err := p.CheckURL(ctx, "https://does-not-exist.invalid/hook"); err != nil {
		t.Errorf("unresolvable host refused: %v", err)
	}
	if err := (Policy{AllowPrivate: true}).CheckURL(ctx, "http://localhost:5678/webhook"); err != nil {
		t.Errorf("AllowPrivate refused localhost: %v", err)
	}
}

// The dialer checks the resolved address, so a name pointing at loopback is
// refused when it connects, whatever it resolved to when it was saved.
func TestDialerBlocksResolvedAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	_, err = Policy{}.Dialer(2*time.Second).Dial("tcp", net.JoinHostPort("localhost", port))
	if err == nil || !strings.Contains(err.Error(), ErrBlocked.Error()) {
		t.Fatalf("want a blocked dial, got %v", err)
	}
	conn, err := Policy{AllowPrivate: true}.Dialer(2*time.Second).Dial("tcp", net.JoinHostPort("localhost", port))
	if err != nil {
		t.Fatalf("allowed dial: %v", err)
	}
	conn.Close()
}

func TestFromEnv(t *testing.T) {
	t.Setenv("X_ALLOW_PRIVATE_NETWORKS", "")
	t.Setenv("X_ALLOWED_NETWORKS", "10.0.5.0/24, 192.168.1.10 ,fd00::/8")
	p, err := FromEnv("X", false)
	if err != nil || p.AllowPrivate || len(p.Allowed) != 3 {
		t.Fatalf("policy %+v, err %v", p, err)
	}
	if p.Check(netip.MustParseAddr("192.168.1.10")) != nil || p.Check(netip.MustParseAddr("192.168.1.11")) == nil {
		t.Error("a single address did not allow exactly itself")
	}

	t.Setenv("X_ALLOW_PRIVATE_NETWORKS", "true")
	t.Setenv("X_ALLOWED_NETWORKS", "nonsense")
	p, err = FromEnv("X", false)
	if err == nil || !p.AllowPrivate {
		t.Fatalf("want an error for a bad entry and AllowPrivate on, got %+v %v", p, err)
	}

	t.Setenv("X_ALLOW_PRIVATE_NETWORKS", "false")
	t.Setenv("X_ALLOWED_NETWORKS", "")
	if p, _ := FromEnv("X", true); p.AllowPrivate {
		t.Error("false did not override the default")
	}
}

// Describe names what went wrong without the address or port.
func TestDescribeHidesAddresses(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // now nothing listens: connection refused

	_, err = Policy{AllowPrivate: true}.Dialer(2*time.Second).Dial("tcp", addr)
	if err == nil {
		t.Skip("the closed port accepted a connection")
	}
	got := Describe(err)
	if got != "the connection was refused" {
		t.Errorf("Describe(%v) = %q", err, got)
	}
	_, err = Policy{}.Dialer(2*time.Second).Dial("tcp", addr)
	if got := Describe(err); got != "blocked: the destination is a private or reserved address" {
		t.Errorf("blocked: %q", got)
	}
}
