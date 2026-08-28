// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package messagescan

import (
	"strings"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/lib/pq"
)

type stubLookup struct {
	suppressed  bool
	fromIP      int64
	fromSender  int64
	filters     []models.MessageFilter
	filterError error
}

func (s *stubLookup) IsSuppressed(uint, string) bool                      { return s.suppressed }
func (s *stubLookup) SubmissionsFromIP(string, time.Time) int64           { return s.fromIP }
func (s *stubLookup) SubmissionsFromSender(uint, string, time.Time) int64 { return s.fromSender }
func (s *stubLookup) ActiveFilters(uint, uint) ([]models.MessageFilter, error) {
	return s.filters, s.filterError
}

func testForm() *models.Form {
	id := uint(1)
	return &models.Form{
		ID:                  1,
		WorkspaceID:         &id,
		ScanEnabled:         true,
		FlagThreshold:       3,
		QuarantineThreshold: 6,
		RejectThreshold:     10,
	}
}

func TestHoneypotRejectsImmediately(t *testing.T) {
	s := New(&stubLookup{})
	v := s.Scan(Input{Form: testForm(), HoneypotHit: true})
	if v.Action != ActionReject {
		t.Fatalf("Action = %s, want reject", v.Action)
	}
	if v.Status() != models.MessageStatusRejected {
		t.Fatalf("Status = %s, want rejected", v.Status())
	}
}

func TestNonceFailureRejectsImmediately(t *testing.T) {
	s := New(&stubLookup{})
	v := s.Scan(Input{Form: testForm(), NonceFailed: true})
	if v.Action != ActionReject {
		t.Fatalf("Action = %s, want reject", v.Action)
	}
}

func TestScanDisabledAllows(t *testing.T) {
	form := testForm()
	form.ScanEnabled = false
	s := New(&stubLookup{suppressed: true})
	v := s.Scan(Input{Form: form, SenderEmail: "spam@mailinator.com", Body: strings.Repeat("BUY NOW ", 40)})
	if v.Action != ActionAllow || v.Score != 0 {
		t.Fatalf("Action = %s score = %v, want allow/0", v.Action, v.Score)
	}
}

func TestThresholdMapping(t *testing.T) {
	cases := []struct {
		score float64
		want  Action
	}{
		{0, ActionAllow},
		{2.9, ActionAllow},
		{3, ActionFlag},
		{5.9, ActionFlag},
		{6, ActionQuarantine},
		{9.9, ActionQuarantine},
		{10, ActionReject},
		{50, ActionReject},
	}
	for _, tc := range cases {
		if got := resolveAction(testForm(), tc.score); got != tc.want {
			t.Fatalf("resolveAction(%v) = %s, want %s", tc.score, got, tc.want)
		}
	}
}

func TestAllowlistShortCircuits(t *testing.T) {
	s := New(&stubLookup{
		suppressed: true,
		filters: []models.MessageFilter{
			{ID: 7, Kind: models.FilterKindDomain, Pattern: "partner.com", Action: models.FilterActionAllowlist},
		},
	})
	v := s.Scan(Input{
		Form:        testForm(),
		SenderEmail: "vip@partner.com",
		Body:        strings.Repeat("http://spam.test ", 20),
	})
	if !v.Allowlisted {
		t.Fatal("expected the message to be allowlisted")
	}
	if v.Action != ActionAllow || v.Score != 0 {
		t.Fatalf("Action = %s score = %v, want allow/0", v.Action, v.Score)
	}
}

func TestFilterActionSetsFloorRegardlessOfScore(t *testing.T) {
	s := New(&stubLookup{
		filters: []models.MessageFilter{
			{ID: 1, Kind: models.FilterKindKeyword, Pattern: "casino", Action: models.FilterActionReject, Score: 0},
		},
	})
	v := s.Scan(Input{Form: testForm(), Body: "come to our casino"})
	if v.Action != ActionReject {
		t.Fatalf("Action = %s, want reject", v.Action)
	}
}

func TestFilterScoreAccumulates(t *testing.T) {
	s := New(&stubLookup{
		filters: []models.MessageFilter{
			{ID: 1, Kind: models.FilterKindKeyword, Pattern: "seo", Action: models.FilterActionScore, Score: 2},
			{ID: 2, Kind: models.FilterKindKeyword, Pattern: "ranking", Action: models.FilterActionScore, Score: 2},
		},
	})
	v := s.Scan(Input{Form: testForm(), Body: "improve your seo and ranking"})
	if v.Score < 4 {
		t.Fatalf("Score = %v, want at least 4", v.Score)
	}
	if len(v.FilterHits) != 2 {
		t.Fatalf("FilterHits = %v, want 2 entries", v.FilterHits)
	}
}

func TestKeywordMatchesWholeWordOnly(t *testing.T) {
	s := New(&stubLookup{
		filters: []models.MessageFilter{
			{ID: 1, Kind: models.FilterKindKeyword, Pattern: "cat", Action: models.FilterActionScore, Score: 5},
		},
	})
	if v := s.Scan(Input{Form: testForm(), Body: "concatenate these"}); len(v.FilterHits) != 0 {
		t.Fatal("keyword filter must not match inside a longer word")
	}
	if v := s.Scan(Input{Form: testForm(), Body: "the cat sat"}); len(v.FilterHits) != 1 {
		t.Fatal("keyword filter should match a standalone word")
	}
}

func TestPhraseMatchesSubstring(t *testing.T) {
	s := New(&stubLookup{
		filters: []models.MessageFilter{
			{ID: 1, Kind: models.FilterKindPhrase, Pattern: "cat", Action: models.FilterActionScore, Score: 5},
		},
	})
	if v := s.Scan(Input{Form: testForm(), Body: "concatenate these"}); len(v.FilterHits) != 1 {
		t.Fatal("phrase filter should match inside a longer word")
	}
}

func TestCaseSensitivity(t *testing.T) {
	sensitive := New(&stubLookup{
		filters: []models.MessageFilter{
			{ID: 1, Kind: models.FilterKindKeyword, Pattern: "Casino", Action: models.FilterActionScore, Score: 5, CaseSensitive: true},
		},
	})
	if v := sensitive.Scan(Input{Form: testForm(), Body: "visit our casino"}); len(v.FilterHits) != 0 {
		t.Fatal("case-sensitive filter matched a differently-cased word")
	}

	insensitive := New(&stubLookup{
		filters: []models.MessageFilter{
			{ID: 1, Kind: models.FilterKindKeyword, Pattern: "Casino", Action: models.FilterActionScore, Score: 5},
		},
	})
	if v := insensitive.Scan(Input{Form: testForm(), Body: "visit our casino"}); len(v.FilterHits) != 1 {
		t.Fatal("case-insensitive filter should have matched")
	}
}

func TestDomainFilterMatchesSubdomains(t *testing.T) {
	s := New(&stubLookup{
		filters: []models.MessageFilter{
			{ID: 1, Kind: models.FilterKindDomain, Pattern: "@spam.test", Action: models.FilterActionScore, Score: 5},
		},
	})
	if v := s.Scan(Input{Form: testForm(), SenderEmail: "a@mail.spam.test"}); len(v.FilterHits) != 1 {
		t.Fatal("domain filter should match a subdomain")
	}
	if v := s.Scan(Input{Form: testForm(), SenderEmail: "a@notspam.test"}); len(v.FilterHits) != 0 {
		t.Fatal("domain filter must not match an unrelated host ending in the same suffix")
	}
}

func TestFilterFieldScoping(t *testing.T) {
	s := New(&stubLookup{
		filters: []models.MessageFilter{
			{
				ID: 1, Kind: models.FilterKindKeyword, Pattern: "urgent",
				Action: models.FilterActionScore, Score: 5,
				Fields: pq.StringArray{"subject"},
			},
		},
	})
	if v := s.Scan(Input{Form: testForm(), Body: "urgent request"}); len(v.FilterHits) != 0 {
		t.Fatal("subject-scoped filter must not match the body")
	}
	if v := s.Scan(Input{Form: testForm(), Subject: "urgent request"}); len(v.FilterHits) != 1 {
		t.Fatal("subject-scoped filter should match the subject")
	}
}

func TestBuiltinSignals(t *testing.T) {
	cases := []struct {
		name   string
		lookup *stubLookup
		input  Input
		reason string
	}{
		{"suppressed sender", &stubLookup{suppressed: true}, Input{SenderEmail: "a@b.com"}, "sender_suppressed"},
		{"disposable domain", &stubLookup{}, Input{SenderEmail: "a@mailinator.com"}, "disposable_email"},
		{"unparseable sender", &stubLookup{}, Input{SenderEmail: "not an address"}, "sender_unparseable"},
		{"shortener", &stubLookup{}, Input{Body: "click https://bit.ly/x"}, "url_shortener"},
		{"short body", &stubLookup{}, Input{Body: "hi"}, "body_too_short"},
		{"shouting", &stubLookup{}, Input{Body: strings.Repeat("LOUD NOISES ", 6)}, "excessive_caps"},
		{"markup", &stubLookup{}, Input{Body: `check <a href="http://x">this</a> out for a while`}, "embedded_markup"},
		{"script", &stubLookup{}, Input{Body: `<script>alert(1)</script> and some filler text here`}, "embedded_script"},
		{"repeat ip", &stubLookup{fromIP: 9}, Input{ClientIP: "203.0.113.7"}, "repeat_ip:9"},
		{"bot agent", &stubLookup{}, Input{UserAgent: "python-requests/2.31"}, "bot_user_agent"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(tc.lookup)
			in := tc.input
			in.Form = testForm()
			v := s.Scan(in)
			for _, r := range v.Reasons {
				if r == tc.reason {
					return
				}
			}
			t.Fatalf("reasons %v do not contain %q", v.Reasons, tc.reason)
		})
	}
}

func TestLinkCountScoreIsCapped(t *testing.T) {
	s := New(&stubLookup{})
	v := s.Scan(Input{Form: testForm(), Body: strings.Repeat("https://x.test ", 60)})
	for _, r := range v.Reasons {
		if strings.HasPrefix(r, "links:") {
			if v.Score > 20 {
				t.Fatalf("Score = %v, link scoring should stay bounded", v.Score)
			}
			return
		}
	}
	t.Fatal("expected a links: reason")
}

func TestValidatePattern(t *testing.T) {
	if err := ValidatePattern(models.FilterKindKeyword, ""); err == nil {
		t.Fatal("empty pattern should be rejected")
	}
	if err := ValidatePattern(models.FilterKindKeyword, strings.Repeat("a", 600)); err == nil {
		t.Fatal("overlong pattern should be rejected")
	}
	if err := ValidatePattern(models.FilterKindRegex, "a(b"); err == nil {
		t.Fatal("invalid regex should be rejected")
	}
	if err := ValidatePattern(models.FilterKindRegex, `^spam\d+$`); err != nil {
		t.Fatalf("valid regex rejected: %v", err)
	}
}

func TestRegexFilterIgnoresOverlongPattern(t *testing.T) {
	s := New(&stubLookup{
		filters: []models.MessageFilter{
			{ID: 1, Kind: models.FilterKindRegex, Pattern: strings.Repeat("a", 600), Action: models.FilterActionReject},
		},
	})
	if v := s.Scan(Input{Form: testForm(), Body: strings.Repeat("a", 600)}); v.Action == ActionReject {
		t.Fatal("an overlong regex pattern must not be compiled or matched")
	}
}

func TestVerdictStatusMapping(t *testing.T) {
	cases := map[Action]models.MessageStatus{
		ActionAllow:      models.MessageStatusReceived,
		ActionFlag:       models.MessageStatusFlagged,
		ActionQuarantine: models.MessageStatusQuarantined,
		ActionReject:     models.MessageStatusRejected,
	}
	for action, want := range cases {
		if got := (Verdict{Action: action}).Status(); got != want {
			t.Fatalf("Verdict{%s}.Status() = %s, want %s", action, got, want)
		}
	}
}
