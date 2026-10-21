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

package messagescan

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/goposta/posta/internal/models"
)

type Action string

const (
	ActionAllow      Action = "allow"
	ActionFlag       Action = "flag"
	ActionQuarantine Action = "quarantine"
	ActionReject     Action = "reject"
)

var actionRank = map[Action]int{
	ActionAllow:      0,
	ActionFlag:       1,
	ActionQuarantine: 2,
	ActionReject:     3,
}

type Verdict struct {
	Score       float64
	Reasons     []string
	Action      Action
	Allowlisted bool
	FilterHits  []uint
}

func (v Verdict) Status() models.MessageStatus {
	switch v.Action {
	case ActionReject:
		return models.MessageStatusRejected
	case ActionQuarantine:
		return models.MessageStatusQuarantined
	case ActionFlag:
		return models.MessageStatusFlagged
	default:
		return models.MessageStatusReceived
	}
}

type Lookup interface {
	IsSuppressed(workspaceID uint, email string) bool
	SubmissionsFromIP(ip string, since time.Time) int64
	SubmissionsFromSender(src models.ScanSource, email string, since time.Time) int64
	ActiveFilters(workspaceID uint, src models.ScanSource) ([]models.MessageFilter, error)
}

// Policy is the per-endpoint scanning configuration: whether to scan, where
// the score thresholds sit, and which source's filters apply. *models.Form
// implements it; a chat widget will too.
type Policy interface {
	ScanningEnabled() bool
	// ScanThresholds returns the flag, quarantine, and reject scores. A
	// non-positive value means the default for that rung.
	ScanThresholds() (flag, quarantine, reject float64)
	ScanSource() models.ScanSource
}

type Input struct {
	Policy      Policy
	WorkspaceID uint
	SenderEmail string
	SenderName  string
	Subject     string
	Body        string
	Fields      []models.MessageField
	ClientIP    string
	UserAgent   string
	HoneypotHit bool
	NonceFailed bool
}

type Scanner struct {
	lookup Lookup

	mu    sync.RWMutex
	cache map[string]*regexp.Regexp
}

func New(lookup Lookup) *Scanner {
	return &Scanner{lookup: lookup, cache: make(map[string]*regexp.Regexp)}
}

func (s *Scanner) Scan(in Input) Verdict {
	v := Verdict{Action: ActionAllow}

	if in.HoneypotHit {
		v.Reasons = append(v.Reasons, "honeypot")
		v.Action = ActionReject
		v.Score = 100
		return v
	}
	if in.NonceFailed {
		v.Reasons = append(v.Reasons, "nonce_invalid")
		v.Action = ActionReject
		v.Score = 100
		return v
	}

	if in.Policy != nil && !in.Policy.ScanningEnabled() {
		return v
	}

	floor := ActionAllow

	if s.lookup != nil {
		hits, err := s.lookup.ActiveFilters(in.WorkspaceID, source(in.Policy))
		if err == nil {
			for i := range hits {
				f := &hits[i]
				matched, reason := s.matchFilter(f, in)
				if !matched {
					continue
				}
				v.FilterHits = append(v.FilterHits, f.ID)
				if f.Action == models.FilterActionAllowlist {
					v.Reasons = []string{reason}
					v.Allowlisted = true
					v.Action = ActionAllow
					v.Score = 0
					return v
				}
				v.Reasons = append(v.Reasons, reason)
				v.Score += f.Score
				if a := filterAction(f.Action); actionRank[a] > actionRank[floor] {
					floor = a
				}
			}
		}
	}

	for _, sig := range builtinSignals {
		score, reason := sig(s, in)
		if score == 0 && reason == "" {
			continue
		}
		v.Score += score
		if reason != "" {
			v.Reasons = append(v.Reasons, reason)
		}
	}

	v.Action = resolveAction(in.Policy, v.Score)
	if actionRank[floor] > actionRank[v.Action] {
		v.Action = floor
	}
	return v
}

func resolveAction(p Policy, score float64) Action {
	flag, quarantine, reject := 3.0, 6.0, 10.0
	if p != nil {
		f, q, r := p.ScanThresholds()
		if f > 0 {
			flag = f
		}
		if q > 0 {
			quarantine = q
		}
		if r > 0 {
			reject = r
		}
	}
	switch {
	case score >= reject:
		return ActionReject
	case score >= quarantine:
		return ActionQuarantine
	case score >= flag:
		return ActionFlag
	default:
		return ActionAllow
	}
}

func filterAction(a models.FilterAction) Action {
	switch a {
	case models.FilterActionFlag:
		return ActionFlag
	case models.FilterActionQuarantine:
		return ActionQuarantine
	case models.FilterActionReject:
		return ActionReject
	default:
		return ActionAllow
	}
}

func source(p Policy) models.ScanSource {
	if p == nil {
		return models.ScanSource{}
	}
	return p.ScanSource()
}

func (s *Scanner) matchFilter(f *models.MessageFilter, in Input) (bool, string) {
	switch f.Kind {
	case models.FilterKindEmail:
		return strings.EqualFold(strings.TrimSpace(in.SenderEmail), strings.TrimSpace(f.Pattern)),
			fmt.Sprintf("filter:email:%s", f.Pattern)
	case models.FilterKindDomain:
		domain := emailDomain(in.SenderEmail)
		pattern := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(f.Pattern), "@"))
		return domain != "" && (domain == pattern || strings.HasSuffix(domain, "."+pattern)),
			fmt.Sprintf("filter:domain:%s", f.Pattern)
	case models.FilterKindIP:
		return in.ClientIP != "" && in.ClientIP == strings.TrimSpace(f.Pattern),
			fmt.Sprintf("filter:ip:%s", f.Pattern)
	case models.FilterKindRegex:
		re := s.compile(f.Pattern, f.CaseSensitive)
		if re == nil {
			return false, ""
		}
		for _, text := range filterTargets(f, in) {
			if re.MatchString(text) {
				return true, fmt.Sprintf("filter:regex:%d", f.ID)
			}
		}
		return false, ""
	case models.FilterKindPhrase:
		needle := f.Pattern
		for _, text := range filterTargets(f, in) {
			if containsText(text, needle, f.CaseSensitive) {
				return true, fmt.Sprintf("filter:phrase:%s", f.Pattern)
			}
		}
		return false, ""
	default:
		needle := f.Pattern
		for _, text := range filterTargets(f, in) {
			if containsWord(text, needle, f.CaseSensitive) {
				return true, fmt.Sprintf("filter:keyword:%s", f.Pattern)
			}
		}
		return false, ""
	}
}

func filterTargets(f *models.MessageFilter, in Input) []string {
	if len(f.Fields) == 0 {
		out := make([]string, 0, 4+len(in.Fields))
		out = append(out, in.Subject, in.Body, in.SenderName, in.SenderEmail)
		for _, fld := range in.Fields {
			out = append(out, fld.Value)
		}
		return out
	}
	var out []string
	for _, want := range f.Fields {
		switch strings.ToLower(want) {
		case "subject":
			out = append(out, in.Subject)
		case "body", "message":
			out = append(out, in.Body)
		case "name":
			out = append(out, in.SenderName)
		case "email":
			out = append(out, in.SenderEmail)
		default:
			for _, fld := range in.Fields {
				if strings.EqualFold(fld.Key, want) {
					out = append(out, fld.Value)
				}
			}
		}
	}
	return out
}

func (s *Scanner) compile(pattern string, caseSensitive bool) *regexp.Regexp {
	if len(pattern) > models.MaxFilterPatternLength {
		return nil
	}
	key := pattern
	if !caseSensitive {
		key = "(?i)" + pattern
	}

	s.mu.RLock()
	re, ok := s.cache[key]
	s.mu.RUnlock()
	if ok {
		return re
	}

	compiled, err := regexp.Compile(key)
	if err != nil {
		compiled = nil
	}

	s.mu.Lock()
	s.cache[key] = compiled
	s.mu.Unlock()
	return compiled
}

func ValidatePattern(kind models.FilterKind, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("pattern is required")
	}
	if len(pattern) > models.MaxFilterPatternLength {
		return fmt.Errorf("pattern exceeds %d characters", models.MaxFilterPatternLength)
	}
	if kind == models.FilterKindRegex {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regular expression: %w", err)
		}
	}
	return nil
}

func (s *Scanner) MatchesFilter(f *models.MessageFilter, in Input) bool {
	matched, _ := s.matchFilter(f, in)
	return matched
}
