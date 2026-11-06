//go:build enterprise

// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

package ldap

import (
	"context"
	"errors"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func provider(cfgs ...models.LDAPConfig) *Provider {
	return New(func(context.Context) ([]models.LDAPConfig, error) { return cfgs, nil }, nil)
}

// An empty password is an anonymous bind most directories accept, so it must never reach one.
func TestEmptyCredentialsNeverBind(t *testing.T) {
	p := provider(models.LDAPConfig{Name: "corp", Host: "ldap.invalid", Enabled: true, UserBaseDN: "dc=corp"})
	for _, tc := range [][2]string{{"alice@corp.test", ""}, {"  ", "secret"}} {
		if _, err := p.Authenticate(context.Background(), tc[0], tc[1]); !errors.Is(err, ErrNoMatch) {
			t.Errorf("Authenticate(%q, %q) = %v, want ErrNoMatch without dialing", tc[0], tc[1], err)
		}
	}
}

func TestNoDirectories(t *testing.T) {
	if _, err := provider().Authenticate(context.Background(), "alice@corp.test", "pw"); !errors.Is(err, ErrNoMatch) {
		t.Fatalf("err = %v, want ErrNoMatch", err)
	}
}

func TestUnreachableDirectoryReportsAnError(t *testing.T) {
	p := provider(models.LDAPConfig{Name: "corp", Host: "127.0.0.1", Port: 1, TimeoutSeconds: 1, TLSMode: models.LDAPTLSNone, Enabled: true, UserBaseDN: "dc=corp"})
	if _, err := p.Authenticate(context.Background(), "alice@corp.test", "pw"); err == nil || errors.Is(err, ErrNoMatch) {
		t.Fatalf("err = %v, want a dial error", err)
	}
}

func TestFilters(t *testing.T) {
	if got := userFilter(&models.LDAPConfig{}); got != "(mail=%s)" {
		t.Errorf("default user filter = %q", got)
	}
	if got := userFilter(&models.LDAPConfig{AttrEmail: "userPrincipalName"}); got != "(userPrincipalName=%s)" {
		t.Errorf("user filter from the email attribute = %q", got)
	}
	if got := groupFilter(&models.LDAPConfig{}, "cn=a,dc=c"); got != "(member=cn=a,dc=c)" {
		t.Errorf("default group filter = %q", got)
	}
	if got := groupFilter(&models.LDAPConfig{NestedGroups: true}, "cn=a,dc=c"); got != "(member:1.2.840.113556.1.4.1941:=cn=a,dc=c)" {
		t.Errorf("nested group filter = %q", got)
	}
	// The user DN is escaped into the filter, so a crafted DN cannot widen it.
	if got := groupFilter(&models.LDAPConfig{GroupFilter: "(uniqueMember=%s)"}, "cn=a*)(x=*"); got != `(uniqueMember=cn=a\2a\29\28x=\2a)` {
		t.Errorf("escaped group filter = %q", got)
	}
}
