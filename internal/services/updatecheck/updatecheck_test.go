/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/goposta/posta/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func rel(tag string, pre, draft bool) Release {
	return Release{TagName: tag, Prerelease: pre, Draft: draft, HTMLURL: "https://example.test/" + tag}
}

// ─── Version selection (no database required) ───

// A lexical compare puts "v0.12.0-rc.9" after "v0.12.0-rc.10", which would offer
// an rc.10 user a downgrade. Ordering must be semver.
func TestNewestPicksBySemverNotString(t *testing.T) {
	releases := []Release{rel("v0.12.0-rc.9", true, false), rel("v0.12.0-rc.10", true, false)}
	got, ok := Newest("v0.12.0-rc.2", releases)
	if !ok || got.TagName != "v0.12.0-rc.10" {
		t.Fatalf("Newest = %q (ok=%v), want v0.12.0-rc.10", got.TagName, ok)
	}
}

func TestPrereleaseUserIsOfferedStable(t *testing.T) {
	releases := []Release{rel("v0.12.0", false, false), rel("v0.12.0-rc.3", true, false)}
	got, ok := Newest("v0.12.0-rc.2", releases)
	if !ok || got.TagName != "v0.12.0" {
		t.Fatalf("Newest = %q (ok=%v), want the stable v0.12.0", got.TagName, ok)
	}
}

// Someone on a stable build must never be nudged onto a release candidate.
func TestStableUserIsNeverOfferedAPrerelease(t *testing.T) {
	releases := []Release{rel("v0.13.0-rc.1", true, false)}
	if got, ok := Newest("v0.12.0", releases); ok {
		t.Fatalf("stable build offered prerelease %q", got.TagName)
	}
}

func TestUpToDateAndOlderReleasesIgnored(t *testing.T) {
	releases := []Release{rel("v0.12.0", false, false), rel("v0.11.0", false, false)}
	if got, ok := Newest("v0.12.0", releases); ok {
		t.Fatalf("up-to-date build offered %q", got.TagName)
	}
}

func TestDraftsIgnored(t *testing.T) {
	releases := []Release{rel("v0.13.0", false, true)}
	if got, ok := Newest("v0.12.0", releases); ok {
		t.Fatalf("draft release offered: %q", got.TagName)
	}
}

// A build with no meaningful version compares against nothing.
func TestDevBuildNeverChecks(t *testing.T) {
	for _, v := range []string{"dev", "", "unknown", "a1b2c3d"} {
		s := &Service{version: v, enabled: true}
		if s.Enabled() {
			t.Errorf("version %q: Enabled() = true, want false", v)
		}
		if _, ok := Newest(v, []Release{rel("v9.9.9", false, false)}); ok {
			t.Errorf("version %q: was offered an upgrade", v)
		}
	}
}

// Posta stamps the tag with a leading "v", but accept a bare version too so a
// differently-built image still compares.
func TestNormalizeHandlesBakedTagWithoutV(t *testing.T) {
	for _, v := range []string{"0.12.0", "v0.12.0", " v0.12.0 "} {
		if got := normalize(v); got != "v0.12.0" {
			t.Errorf("normalize(%q) = %q, want v0.12.0", v, got)
		}
	}
}

func TestIsNewerRejectsOlderAndEqual(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.12.0", "v0.13.0", true},
		{"v0.12.0", "v0.12.0", false},
		{"v0.13.0", "v0.12.1", false}, // the upgraded-install case
		{"dev", "v0.13.0", false},
		{"v0.12.0", "", false},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.current, tc.latest); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

// ─── Check against a stub GitHub (requires a database) ───

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=posta password=posta dbname=posta port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Skipf("skipping: no test database available: %v", err)
	}
	if err := db.AutoMigrate(&models.UpdateStatus{}); err != nil {
		t.Skipf("skipping: cannot migrate update_status schema: %v", err)
	}
	// Each test owns the singleton row, so clear it rather than inherit a verdict.
	db.Exec("DELETE FROM update_statuses")
	return db
}

func stubGitHub(t *testing.T, releases []Release, etag string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if etag != "" {
			if r.Header.Get("If-None-Match") == etag {
				w.Header().Set("ETag", etag)
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", etag)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckStoresNewerVersion(t *testing.T) {
	db := testDB(t)
	srv := stubGitHub(t, []Release{rel("v0.13.0", false, false)}, "")

	s := NewService(db, "v0.12.0", true)
	s.setBaseURL(srv.URL)
	if err := s.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}

	st, err := s.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.LatestVersion != "v0.13.0" {
		t.Errorf("LatestVersion = %q, want v0.13.0", st.LatestVersion)
	}
	if st.LastError != "" {
		t.Errorf("LastError = %q, want empty", st.LastError)
	}
	if st.CheckedAt == nil {
		t.Error("CheckedAt not recorded")
	}
}

// The regression that motivates CheckedVersion: after an upgrade the release
// list is unchanged, so replaying the ETag earns a 304 and would preserve a
// verdict computed for the *previous* build — telling a v0.13.0 install that
// v0.13.0 is available.
func TestUpgradedBuildIsNotOfferedTheReleaseItAlreadyPassed(t *testing.T) {
	db := testDB(t)
	releases := []Release{rel("v0.13.0", false, false)}
	srv := stubGitHub(t, releases, `W/"list-v1"`)

	old := NewService(db, "v0.12.0", true)
	old.setBaseURL(srv.URL)
	if err := old.Check(context.Background()); err != nil {
		t.Fatalf("first Check: %v", err)
	}

	// Same list, but the running build is now the release itself.
	upgraded := NewService(db, "v0.13.0", true)
	upgraded.setBaseURL(srv.URL)
	if err := upgraded.Check(context.Background()); err != nil {
		t.Fatalf("second Check: %v", err)
	}

	st, err := upgraded.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.LatestVersion != "" {
		t.Errorf("LatestVersion = %q, want empty — the build is already at the newest release", st.LatestVersion)
	}
	// And the read-time guard independently refuses to offer it.
	if IsNewer("v0.13.0", "v0.13.0") {
		t.Error("IsNewer offered an equal version")
	}
}

func TestCheckReplaysETagAndKeepsCacheOn304(t *testing.T) {
	db := testDB(t)
	srv := stubGitHub(t, []Release{rel("v0.13.0", false, false)}, `W/"list-v1"`)

	s := NewService(db, "v0.12.0", true)
	s.setBaseURL(srv.URL)
	if err := s.Check(context.Background()); err != nil {
		t.Fatalf("first Check: %v", err)
	}
	// Second check sends If-None-Match and gets a 304; the verdict must survive.
	if err := s.Check(context.Background()); err != nil {
		t.Fatalf("second Check: %v", err)
	}

	st, _ := s.Status()
	if st.LatestVersion != "v0.13.0" {
		t.Errorf("LatestVersion = %q after 304, want v0.13.0 preserved", st.LatestVersion)
	}
}

func TestCheckClearsStalePointerWhenUpToDate(t *testing.T) {
	db := testDB(t)

	// Seed a verdict from an earlier release.
	seeded := NewService(db, "v0.12.0", true)
	srv1 := stubGitHub(t, []Release{rel("v0.13.0", false, false)}, "")
	seeded.setBaseURL(srv1.URL)
	if err := seeded.Check(context.Background()); err != nil {
		t.Fatalf("seed Check: %v", err)
	}

	// Now the running build is current and no newer release exists.
	srv2 := stubGitHub(t, []Release{rel("v0.13.0", false, false)}, "")
	s := NewService(db, "v0.13.0", true)
	s.setBaseURL(srv2.URL)
	if err := s.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}

	st, _ := s.Status()
	if st.LatestVersion != "" || st.ReleaseURL != "" || st.PublishedAt != nil {
		t.Errorf("stale pointer left behind: version=%q url=%q published=%v",
			st.LatestVersion, st.ReleaseURL, st.PublishedAt)
	}
}

// An air-gapped install fails this daily. It must record the failure rather than
// return an error that would paint the jobs page red forever.
func TestCheckRecordsErrorWithoutFailing(t *testing.T) {
	db := testDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	s := NewService(db, "v0.12.0", true)
	s.setBaseURL(srv.URL)
	if err := s.Check(context.Background()); err != nil {
		t.Fatalf("Check returned an error instead of recording it: %v", err)
	}
	st, _ := s.Status()
	if st.LastError == "" {
		t.Error("LastError not recorded for a failing check")
	}
}

// Dismissal is per-version so the next release notifies again.
func TestDismissIsPerVersion(t *testing.T) {
	db := testDB(t)
	s := NewService(db, "v0.12.0", true)

	if err := s.Dismiss("v0.13.0"); err != nil {
		t.Fatalf("Dismiss: %v", err)
	}
	st, _ := s.Status()
	if st.DismissedVersion != "v0.13.0" {
		t.Fatalf("DismissedVersion = %q, want v0.13.0", st.DismissedVersion)
	}
	// A later release is a different version, so it is not silenced.
	if st.DismissedVersion == "v0.14.0" {
		t.Error("a newer version was silenced by an older dismissal")
	}
}

// A disabled checker performs no request at all.
func TestDisabledCheckIsANoOp(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	s := NewService(nil, "v0.12.0", false) // nil db: reaching it would panic
	s.setBaseURL(srv.URL)
	if err := s.Check(context.Background()); err != nil {
		t.Fatalf("disabled Check returned an error: %v", err)
	}
	if called {
		t.Error("disabled checker contacted the release API")
	}
}
