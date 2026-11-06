//go:build enterprise

// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

package enterprise

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	eldap "github.com/goposta/posta/internal/enterprise/ldap"
	"github.com/goposta/posta/internal/enterprise/license"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// Set at link time via -ldflags -X ...enterprise.embeddedKeyring=<base64>; a build without one runs as Community.
var embeddedKeyring string

// Bounds staleness: another replica, or the server when this is the worker, may install or remove the license.
const reloadEvery = time.Minute

func New(db *gorm.DB, opts Options) EE {
	e := &impl{db: db, opts: opts, ldap: eldap.New(opts.LDAPDirectories, opts.Decrypt)}
	if ring, err := license.ParseKeyring(embeddedKeyring); err != nil || ring.Product != Product {
		logger.Warn("enterprise: this build has no license keyring; running as Community")
	} else {
		e.ring = &ring
	}
	if err := e.reload(); err != nil {
		logger.Warn("enterprise: failed to load the installed license", "error", err)
	}
	if e.current().token == "" {
		e.installFromOptions()
	}
	if s := e.Status(); s.State != StateNone {
		logger.Info("enterprise: license loaded", "state", s.State, "customer", s.Customer, "not_after", s.NotAfter)
	}
	return e
}

type impl struct {
	db   *gorm.DB
	ring *license.Keyring
	opts Options
	ldap *eldap.Provider

	mu       sync.RWMutex
	loaded   loadedLicense
	loadedAt time.Time
}

type loadedLicense struct {
	token      string
	claims     *license.Claims // nil when there is none, or it does not verify
	unknownKey bool            // stored, but signed by a key this build does not trust
	binding    string          // the binding that failed, if any
}

func (e *impl) installFromOptions() {
	token, source := strings.TrimSpace(e.opts.License), "POSTA_LICENSE"
	if token == "" && e.opts.LicenseFile != "" {
		raw, err := os.ReadFile(e.opts.LicenseFile)
		if err != nil {
			logger.Warn("enterprise: cannot read the license file", "file", e.opts.LicenseFile, "error", err)
			return
		}
		token, source = strings.TrimSpace(string(raw)), e.opts.LicenseFile
	}
	if token == "" {
		return
	}
	if _, err := e.Install(context.Background(), token, ""); err != nil {
		logger.Warn("enterprise: cannot install the configured license", "source", source, "error", err)
	}
}

// The token is authoritative, so a license row edited in the database does not verify.
func (e *impl) reload() error {
	// Find, not First: no license is the usual case, not an error to log.
	var rows []models.License
	if err := e.db.Order("id DESC").Limit(1).Find(&rows).Error; err != nil {
		// Keep what was loaded: a database hiccup must not switch a licensed deployment off.
		e.mu.Lock()
		e.loadedAt = time.Now()
		e.mu.Unlock()
		return err
	}
	next := loadedLicense{}
	if len(rows) > 0 {
		next.token = strings.TrimSpace(rows[0].Token)
	}
	e.mu.RLock()
	unchanged := e.loaded.token == next.token && !e.loadedAt.IsZero()
	e.mu.RUnlock()
	if unchanged {
		// Same token, same keyring, same bindings: same answer.
		e.mu.Lock()
		e.loadedAt = time.Now()
		e.mu.Unlock()
		return nil
	}
	if next.token != "" {
		next.claims, next.unknownKey, next.binding = e.verify(next.token, "")
	}
	e.mu.Lock()
	e.loaded, e.loadedAt = next, time.Now()
	e.mu.Unlock()
	return nil
}

// requestHost counts for the URL binding at install time only; there is no request at boot.
func (e *impl) verify(token, requestHost string) (claims *license.Claims, unknownKey bool, binding string) {
	if e.ring == nil {
		return nil, false, ""
	}
	c, err := license.Verify(*e.ring, Product, token)
	if errors.Is(err, license.ErrUnknownKey) {
		logger.Warn("enterprise: the stored license is signed by a key this build does not trust", "kid", license.KeyID(token))
		return nil, true, ""
	}
	if err != nil {
		logger.Warn("enterprise: the stored license does not verify; running as Community", "error", err)
		return nil, false, ""
	}
	return &c, false, e.bindingFailure(c, requestHost)
}

func (e *impl) current() loadedLicense {
	e.mu.RLock()
	l, stale := e.loaded, time.Since(e.loadedAt) > reloadEvery
	e.mu.RUnlock()
	if !stale {
		return l
	}
	if err := e.reload(); err != nil {
		logger.Warn("enterprise: failed to reload the license", "error", err)
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// Both bindings must hold: the Install ID exactly, the URL against the configured web URL or the request host.
func (e *impl) bindingFailure(c license.Claims, requestHost string) string {
	if id := strings.TrimSpace(c.InstallID); id != "" && !strings.EqualFold(id, strings.TrimSpace(e.opts.InstallID)) {
		return BindingInstallID
	}
	if u := strings.TrimSpace(c.URL); u != "" && !urlAllowed(u, e.opts.InstanceURL, requestHost) {
		return BindingURL
	}
	return ""
}

// Fails open with no candidate, so a deployment without POSTA_WEB_URL is never locked out by a missing setting.
func urlAllowed(licenseURL string, candidates ...string) bool {
	want := hostOf(licenseURL)
	if want == "" {
		return true
	}
	known := false
	for _, c := range candidates {
		h := hostOf(c)
		if h == "" {
			continue
		}
		known = true
		if h == want {
			return true
		}
	}
	return !known
}

func hostOf(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	if u, err := url.Parse(s); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return s
}

func (e *impl) Status() Status {
	l := e.current()
	s := Status{Edition: EditionCommunity, State: StateNone, Build: EditionEnterprise}
	switch {
	case l.unknownKey:
		s.State = StateUnknownKey
		return s
	case l.claims == nil:
		return s
	}
	c := l.claims
	state, graceEnds := license.Evaluate(*c, time.Now())
	notAfter := c.NotAfter
	s.Edition, s.State = EditionEnterprise, string(state)
	s.LicenseID, s.Customer, s.Trial = c.LicenseID, c.Customer, c.Trial
	s.InstallID, s.URL = c.InstallID, c.URL
	s.NotAfter, s.GraceEnds = &notAfter, &graceEnds
	if l.binding != "" {
		s.State, s.BindingError = StateBindingMismatch, l.binding
	}
	return s
}

func (e *impl) Licensed() bool { return e.Status().Licensed() }

// On while degraded too: an expired license freezes settings but never locks people out.
func (e *impl) LDAP() LDAPAuthenticator {
	if !e.Licensed() {
		return nil
	}
	return ldapAdapter{e.ldap}
}

// The ldap package does not import this one, so its types are mapped here.
type ldapAdapter struct{ p *eldap.Provider }

func (a ldapAdapter) Authenticate(ctx context.Context, email, password string) (LDAPIdentity, error) {
	id, err := a.p.Authenticate(ctx, email, password)
	if errors.Is(err, eldap.ErrNoMatch) {
		return LDAPIdentity{}, ErrLDAPNoMatch
	}
	if err != nil {
		return LDAPIdentity{}, err
	}
	return LDAPIdentity{Email: id.Email, Name: id.Name, Groups: id.Groups, Directory: id.Directory}, nil
}

func (a ldapAdapter) TestConnection(_ context.Context, cfg *models.LDAPConfig) LDAPTestResult {
	r := a.p.TestConnection(cfg)
	return LDAPTestResult{OK: r.OK, Message: r.Message, Error: r.Error}
}
func (e *impl) Mutable() bool         { return e.Status().Mutable() }
func (e *impl) Require() error        { return gate(e.Status()) }
func (e *impl) RequireMutable() error { return gateMutable(e.Status()) }

// A license bound to another installation is refused rather than stored.
func (e *impl) Install(ctx context.Context, token, requestHost string) (Status, error) {
	token = strings.TrimSpace(token)
	if e.ring == nil {
		return Status{}, errors.New("this build has no license keyring, so it cannot verify licenses")
	}
	c, err := license.Verify(*e.ring, Product, token)
	if errors.Is(err, license.ErrUnknownKey) {
		return Status{}, ErrUnknownSigningKey
	}
	if err != nil {
		return Status{}, fmt.Errorf("invalid license: %w", err)
	}
	if c.Edition != EditionEnterprise {
		return Status{}, fmt.Errorf("invalid license: edition %q", c.Edition)
	}
	if e.bindingFailure(c, requestHost) != "" {
		return Status{}, ErrLicenseBindingMismatch
	}
	row := models.License{LicenseID: c.LicenseID, Customer: c.Customer, Token: token, NotAfter: c.NotAfter}
	err = e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&models.License{}).Error; err != nil {
			return err
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return Status{}, err
	}
	if err := e.reload(); err != nil {
		return Status{}, err
	}
	return e.Status(), nil
}

func (e *impl) Remove(ctx context.Context) error {
	if err := e.db.WithContext(ctx).Where("1 = 1").Delete(&models.License{}).Error; err != nil {
		return err
	}
	return e.reload()
}
