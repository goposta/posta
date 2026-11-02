//go:build enterprise

// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

// Package ldap authenticates against LDAP and Active Directory with
// go-ldap/ldap/v3. It is compiled only into the Enterprise build and holds no
// user or session code: Authenticate returns a plain identity, and the core
// creates the account and maps groups.
package ldap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
)

// ErrNoMatch means no enabled directory knows the user.
var ErrNoMatch = errors.New("ldap: no matching directory")

// Identity is a directory user whose password bound.
type Identity struct {
	Email     string
	Name      string
	Groups    []string
	Directory string
}

// Result is the outcome of TestConnection.
type Result struct {
	OK      bool
	Message string
	Error   string
}

// Provider binds credentials against the configured directories.
type Provider struct {
	directories func(ctx context.Context) ([]models.LDAPConfig, error)
	decrypt     func(string) (string, error)
}

// New builds the provider. directories lists the enabled directories in the
// order sign-in tries them; decrypt reads their stored service passwords.
func New(directories func(ctx context.Context) ([]models.LDAPConfig, error), decrypt func(string) (string, error)) *Provider {
	if decrypt == nil {
		decrypt = func(s string) (string, error) { return s, nil }
	}
	return &Provider{directories: directories, decrypt: decrypt}
}

// Authenticate tries each enabled directory in order and returns the first
// whose search finds the user and whose bind accepts the password. An empty
// password is refused before any bind: most directories accept an empty bind
// as anonymous, the classic LDAP authentication bypass.
func (p *Provider) Authenticate(ctx context.Context, email, password string) (Identity, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" || p.directories == nil {
		return Identity{}, ErrNoMatch
	}
	cfgs, err := p.directories(ctx)
	if err != nil {
		return Identity{}, fmt.Errorf("ldap: load directories: %w", err)
	}
	var lastErr error
	for i := range cfgs {
		ident, found, err := p.authOne(&cfgs[i], email, password)
		if err != nil {
			// A wrong password or an unreachable directory: try the others, and
			// report this only if none of them knows the user.
			lastErr = err
			logger.Warn("ldap: sign-in attempt failed", "directory", cfgs[i].Name, "error", err)
			continue
		}
		if found {
			return ident, nil
		}
	}
	if lastErr != nil {
		return Identity{}, lastErr
	}
	return Identity{}, ErrNoMatch
}

// authOne searches one directory for the user, then binds as them. found is
// false when the directory does not know the user; an error means it does
// but the bind failed, or the directory could not be reached.
func (p *Provider) authOne(cfg *models.LDAPConfig, email, password string) (Identity, bool, error) {
	conn, err := p.dial(cfg)
	if err != nil {
		return Identity{}, false, err
	}
	defer func() { _ = conn.Close() }()
	if err := p.serviceBind(conn, cfg); err != nil {
		return Identity{}, false, err
	}

	entry, err := searchUser(conn, cfg, email)
	if err != nil || entry == nil {
		return Identity{}, false, err
	}

	// A fresh connection for the user bind, so it does not inherit the
	// service account's bind.
	userConn, err := p.dial(cfg)
	if err != nil {
		return Identity{}, false, err
	}
	defer func() { _ = userConn.Close() }()
	if err := userConn.Bind(entry.DN, password); err != nil {
		return Identity{}, false, fmt.Errorf("ldap: user bind failed: %w", err)
	}

	ident := Identity{
		Directory: cfg.Name,
		Email:     strings.TrimSpace(entry.GetAttributeValue(attrOr(cfg.AttrEmail, "mail"))),
		Name:      strings.TrimSpace(entry.GetAttributeValue(attrOr(cfg.AttrName, "displayName"))),
	}
	if ident.Email == "" {
		ident.Email = email
	}
	// Groups are read over the service connection: users often may not search
	// groups themselves.
	ident.Groups = resolveGroups(conn, cfg, entry)
	return ident, true, nil
}

func (p *Provider) serviceBind(conn *ldapv3.Conn, cfg *models.LDAPConfig) error {
	if cfg.BindDN == "" {
		return nil
	}
	pw, err := p.decrypt(cfg.BindPasswordEnc)
	if err != nil {
		return fmt.Errorf("ldap: read the service password: %w", err)
	}
	if err := conn.Bind(cfg.BindDN, pw); err != nil {
		return fmt.Errorf("ldap: service bind failed: %w", err)
	}
	return nil
}

// searchUser finds the user's entry with the configured filter, the email
// escaped against LDAP filter injection. nil means not found.
func searchUser(conn *ldapv3.Conn, cfg *models.LDAPConfig, email string) (*ldapv3.Entry, error) {
	filter := strings.ReplaceAll(userFilter(cfg), "%s", ldapv3.EscapeFilter(email))
	attrs := dedupe([]string{attrOr(cfg.AttrEmail, "mail"), attrOr(cfg.AttrName, "displayName"), attrOr(cfg.MemberAttr, "memberOf")})
	req := ldapv3.NewSearchRequest(cfg.UserBaseDN, ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases,
		2, timeoutSeconds(cfg), false, filter, attrs, nil)
	res, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("ldap: user search: %w", err)
	}
	switch len(res.Entries) {
	case 0:
		return nil, nil
	case 1:
		return res.Entries[0], nil
	default:
		// Two entries for one sign-in name: binding the first would be a guess.
		return nil, fmt.Errorf("ldap: the user filter matches more than one entry for %q", email)
	}
}

// resolveGroups returns the user's group DNs: the member attribute of their
// entry, or a group search when a group base or filter is configured, or when
// Active Directory nested groups are wanted. A failure yields the attribute's
// groups rather than blocking sign-in.
func resolveGroups(conn *ldapv3.Conn, cfg *models.LDAPConfig, entry *ldapv3.Entry) []string {
	direct := entry.GetAttributeValues(attrOr(cfg.MemberAttr, "memberOf"))
	if !cfg.NestedGroups && cfg.GroupBaseDN == "" && cfg.GroupFilter == "" {
		return direct
	}
	req := ldapv3.NewSearchRequest(orStr(cfg.GroupBaseDN, cfg.UserBaseDN), ldapv3.ScopeWholeSubtree,
		ldapv3.NeverDerefAliases, 0, timeoutSeconds(cfg), false, groupFilter(cfg, entry.DN), []string{"dn"}, nil)
	res, err := conn.Search(req)
	if err != nil {
		logger.Warn("ldap: group search failed", "directory", cfg.Name, "error", err)
		return direct
	}
	out := make([]string, 0, len(res.Entries))
	for _, e := range res.Entries {
		out = append(out, e.DN)
	}
	return out
}

func userFilter(cfg *models.LDAPConfig) string {
	if f := strings.TrimSpace(cfg.UserFilter); f != "" {
		return f
	}
	return "(" + attrOr(cfg.AttrEmail, "mail") + "=%s)"
}

func groupFilter(cfg *models.LDAPConfig, userDN string) string {
	esc := ldapv3.EscapeFilter(userDN)
	if f := strings.TrimSpace(cfg.GroupFilter); f != "" {
		return strings.ReplaceAll(f, "%s", esc)
	}
	if cfg.NestedGroups {
		// Active Directory: LDAP_MATCHING_RULE_IN_CHAIN expands nested groups.
		return "(member:1.2.840.113556.1.4.1941:=" + esc + ")"
	}
	return "(member=" + esc + ")"
}

// dial connects with the configured TLS mode and optional private CA.
func (p *Provider) dial(cfg *models.LDAPConfig) (*ldapv3.Conn, error) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return nil, errors.New("ldap: host is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 389
		if cfg.TLSMode == models.LDAPTLSLDAPS {
			port = 636
		}
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	timeout := time.Duration(timeoutSeconds(cfg)) * time.Second
	dialer := &net.Dialer{Timeout: timeout}

	var conn *ldapv3.Conn
	var err error
	if cfg.TLSMode == models.LDAPTLSLDAPS {
		conn, err = ldapv3.DialURL("ldaps://"+addr, ldapv3.DialWithDialer(dialer), ldapv3.DialWithTLSConfig(tlsConfig(cfg, host)))
	} else {
		conn, err = ldapv3.DialURL("ldap://"+addr, ldapv3.DialWithDialer(dialer))
	}
	if err != nil {
		return nil, fmt.Errorf("ldap: dial %s: %w", addr, err)
	}
	conn.SetTimeout(timeout)
	if cfg.TLSMode == models.LDAPTLSStartTLS {
		if err := conn.StartTLS(tlsConfig(cfg, host)); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("ldap: starttls: %w", err)
		}
	}
	return conn, nil
}

func tlsConfig(cfg *models.LDAPConfig, host string) *tls.Config {
	t := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.InsecureSkipTLS} //nolint:gosec // opt-in, warned about in the dashboard
	if pem := strings.TrimSpace(cfg.CACertPEM); pem != "" {
		pool := x509.NewCertPool()
		if pool.AppendCertsFromPEM([]byte(pem)) {
			t.RootCAs = pool
		}
	}
	return t
}

// TestConnection connects, binds as the service account and reads the user
// base DN. A failure is reported in the result, not as an error.
func (p *Provider) TestConnection(cfg *models.LDAPConfig) Result {
	conn, err := p.dial(cfg)
	if err != nil {
		return Result{Error: err.Error()}
	}
	defer func() { _ = conn.Close() }()
	if err := p.serviceBind(conn, cfg); err != nil {
		return Result{Error: err.Error()}
	}
	if cfg.UserBaseDN == "" {
		return Result{OK: true, Message: "Connected and bound."}
	}
	req := ldapv3.NewSearchRequest(cfg.UserBaseDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases,
		1, timeoutSeconds(cfg), false, "(objectClass=*)", []string{"dn"}, nil)
	if _, err := conn.Search(req); err != nil {
		return Result{Error: "the user base DN cannot be read: " + err.Error()}
	}
	return Result{OK: true, Message: "Connected, bound, and the user base DN is readable."}
}

func timeoutSeconds(cfg *models.LDAPConfig) int {
	if cfg.TimeoutSeconds > 0 {
		return cfg.TimeoutSeconds
	}
	return 10
}

func attrOr(v, def string) string {
	if s := strings.TrimSpace(v); s != "" {
		return s
	}
	return def
}

func orStr(v, def string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
