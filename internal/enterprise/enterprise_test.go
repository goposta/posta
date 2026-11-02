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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/goposta/posta/internal/enterprise/license"
	"github.com/goposta/posta/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const testKID = "posta-202611-test"

// testTx opens the test database and returns a transaction rolled back after
// the test, so the license table of a shared database is never touched.
func testTx(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=posta password=posta dbname=posta port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Skipf("skipping: no test database available: %v", err)
	}
	if err := db.AutoMigrate(&models.License{}); err != nil {
		t.Skipf("skipping: cannot migrate the license table: %v", err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	if err := tx.Where("1 = 1").Delete(&models.License{}).Error; err != nil {
		t.Fatal(err)
	}
	return tx
}

// withKeyring builds this test's keyring into the package, as -ldflags
// would, and returns the private key licenses are signed with.
func withKeyring(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{
		"product": Product,
		"keys":    map[string]string{testKID: base64.StdEncoding.EncodeToString(pub)},
	})
	prev := embeddedKeyring
	embeddedKeyring = base64.StdEncoding.EncodeToString(raw)
	t.Cleanup(func() { embeddedKeyring = prev })
	return priv
}

func sign(t *testing.T, kid string, priv ed25519.PrivateKey, c license.Claims) string {
	t.Helper()
	if c.Product == "" {
		c.Product = Product
	}
	if c.Edition == "" {
		c.Edition = EditionEnterprise
	}
	payload, _ := json.Marshal(c)
	body := Product + "-" + license.Version + "." + kid + "." + base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(body)))
}

func expiringIn(d time.Duration) license.Claims {
	return license.Claims{LicenseID: "lic_test", Customer: "Acme", NotAfter: time.Now().Add(d), GraceDays: 14}
}

func TestLifecycle(t *testing.T) {
	db := testTx(t)
	priv := withKeyring(t)
	ee := New(db, Options{InstallID: "pst_here"})
	ctx := context.Background()

	if s := ee.Status(); s.State != StateNone || s.Build != EditionEnterprise || ee.Licensed() {
		t.Fatalf("before install: %+v", s)
	}
	if !errors.Is(ee.Require(), ErrLicenseRequired) {
		t.Fatalf("require before install: %v", ee.Require())
	}

	s, err := ee.Install(ctx, sign(t, testKID, priv, expiringIn(365*24*time.Hour)), "")
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateValid || s.Customer != "Acme" || !ee.Licensed() || !ee.Mutable() || ee.RequireMutable() != nil {
		t.Fatalf("after install: %+v", s)
	}

	if err := ee.Remove(ctx); err != nil {
		t.Fatal(err)
	}
	if ee.Licensed() || ee.Status().State != StateNone {
		t.Fatalf("after remove: %+v", ee.Status())
	}
}

func TestExpiry(t *testing.T) {
	db := testTx(t)
	priv := withKeyring(t)
	ee := New(db, Options{})
	ctx := context.Background()

	if _, err := ee.Install(ctx, sign(t, testKID, priv, expiringIn(-24*time.Hour)), ""); err != nil {
		t.Fatal(err)
	}
	if s := ee.Status(); s.State != StateGrace || !ee.Mutable() {
		t.Fatalf("in grace: %+v", s)
	}

	if _, err := ee.Install(ctx, sign(t, testKID, priv, expiringIn(-30*24*time.Hour)), ""); err != nil {
		t.Fatal(err)
	}
	// Degraded: Enterprise features keep running, their settings freeze.
	if s := ee.Status(); s.State != StateDegraded || !ee.Licensed() || ee.Mutable() {
		t.Fatalf("degraded: %+v", s)
	}
	if ee.Require() != nil || !errors.Is(ee.RequireMutable(), ErrLicenseExpired) {
		t.Fatalf("degraded gates: %v, %v", ee.Require(), ee.RequireMutable())
	}
}

func TestBindings(t *testing.T) {
	db := testTx(t)
	priv := withKeyring(t)
	ctx := context.Background()
	ee := New(db, Options{InstallID: "pst_here", InstanceURL: "https://mail.example.com"})

	elsewhere := expiringIn(time.Hour)
	elsewhere.InstallID = "pst_elsewhere"
	if _, err := ee.Install(ctx, sign(t, testKID, priv, elsewhere), ""); !errors.Is(err, ErrLicenseBindingMismatch) {
		t.Fatalf("other install id: %v", err)
	}

	bound := expiringIn(time.Hour)
	bound.InstallID, bound.URL = "PST_HERE", "https://Mail.Example.com/"
	if s, err := ee.Install(ctx, sign(t, testKID, priv, bound), ""); err != nil || s.State != StateValid {
		t.Fatalf("matching bindings: %+v, %v", s, err)
	}

	// The same database behind another installation grants nothing.
	moved := New(db, Options{InstallID: "pst_restored_copy", InstanceURL: "https://mail.example.com"})
	if s := moved.Status(); s.State != StateBindingMismatch || s.BindingError != BindingInstallID || moved.Licensed() {
		t.Fatalf("restored elsewhere: %+v", s)
	}
	if !errors.Is(moved.Require(), ErrLicenseBindingMismatch) {
		t.Fatalf("restored elsewhere require: %v", moved.Require())
	}

	other := expiringIn(time.Hour)
	other.URL = "posta.other.org"
	if _, err := ee.Install(ctx, sign(t, testKID, priv, other), "mail.example.com"); !errors.Is(err, ErrLicenseBindingMismatch) {
		t.Fatalf("other url: %v", err)
	}
}

func TestStoredLicense(t *testing.T) {
	db := testTx(t)
	priv := withKeyring(t)

	t.Run("a license from the environment is installed at boot", func(t *testing.T) {
		ee := New(db, Options{License: sign(t, testKID, priv, expiringIn(time.Hour))})
		if !ee.Licensed() {
			t.Fatalf("got %+v", ee.Status())
		}
	})

	t.Run("a stored license wins over the environment", func(t *testing.T) {
		later := expiringIn(time.Hour)
		later.Customer = "Someone else"
		ee := New(db, Options{License: sign(t, testKID, priv, later)})
		if c := ee.Status().Customer; c != "Acme" {
			t.Fatalf("customer %q", c)
		}
	})

	t.Run("an unknown signing key is reported, not mistaken for none", func(t *testing.T) {
		_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
		db.Where("1 = 1").Delete(&models.License{})
		db.Create(&models.License{Token: sign(t, "posta-209901-new", otherPriv, expiringIn(time.Hour)), NotAfter: time.Now()})
		ee := New(db, Options{})
		if s := ee.Status(); s.State != StateUnknownKey || ee.Licensed() || !errors.Is(ee.Require(), ErrUnknownSigningKey) {
			t.Fatalf("got %+v", s)
		}
	})

	t.Run("an edited row does not verify", func(t *testing.T) {
		db.Where("1 = 1").Delete(&models.License{})
		token := sign(t, testKID, priv, expiringIn(time.Hour))
		db.Create(&models.License{Token: token[:len(token)-4] + "AAAA", NotAfter: time.Now()})
		if ee := New(db, Options{}); ee.Licensed() {
			t.Fatalf("got %+v", ee.Status())
		}
	})

	t.Run("another process's install is seen after a reload", func(t *testing.T) {
		db.Where("1 = 1").Delete(&models.License{})
		worker := New(db, Options{}).(*impl)
		server := New(db, Options{})
		if _, err := server.Install(context.Background(), sign(t, testKID, priv, expiringIn(time.Hour)), ""); err != nil {
			t.Fatal(err)
		}
		if worker.Licensed() {
			t.Fatal("the worker reloaded before reloadEvery")
		}
		worker.loadedAt = time.Now().Add(-2 * reloadEvery)
		if !worker.Licensed() {
			t.Fatalf("after reload: %+v", worker.Status())
		}
	})
}

func TestNoKeyring(t *testing.T) {
	db := testTx(t)
	prev := embeddedKeyring
	embeddedKeyring = ""
	t.Cleanup(func() { embeddedKeyring = prev })

	ee := New(db, Options{})
	if ee.Licensed() || ee.Status().Build != EditionEnterprise {
		t.Fatalf("got %+v", ee.Status())
	}
	if _, err := ee.Install(context.Background(), "posta-v1.k.e30.c2ln", ""); err == nil {
		t.Fatal("installed without a keyring")
	}
}
