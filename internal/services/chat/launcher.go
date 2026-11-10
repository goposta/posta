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

package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
)

const (
	maxLauncherLabel = 40
	maxLauncherShift = 200
	maxTeaserText    = 140
	maxTeaserDelay   = 120
	maxTeaserPaths   = 20
	maxTeaserPath    = 200
)

func NormalizeLauncher(l models.ChatLauncher) (models.ChatLauncher, error) {
	out := models.ChatLauncher{Label: CleanLine(l.Label, maxLauncherLabel), HideOnMobile: l.HideOnMobile}
	switch l.Icon {
	case "", models.ChatLauncherIconChat:
	case models.ChatLauncherIconHelp, models.ChatLauncherIconMail:
		out.Icon = l.Icon
	default:
		return out, fmt.Errorf("launcher.icon must be chat, help or mail")
	}
	if l.OffsetX < 0 || l.OffsetX > maxLauncherShift || l.OffsetY < 0 || l.OffsetY > maxLauncherShift {
		return out, fmt.Errorf("launcher offsets must be between 0 and %d pixels", maxLauncherShift)
	}
	out.OffsetX, out.OffsetY = l.OffsetX, l.OffsetY
	return out, nil
}

func NormalizeTeaser(t models.ChatTeaser) (models.ChatTeaser, error) {
	out := models.ChatTeaser{Enabled: t.Enabled, Text: CleanLine(t.Text, maxTeaserText)}
	if t.DelaySeconds < 0 || t.DelaySeconds > maxTeaserDelay {
		return out, fmt.Errorf("teaser.delay_seconds must be between 0 and %d", maxTeaserDelay)
	}
	out.DelaySeconds = t.DelaySeconds
	if out.Enabled && out.Text == "" {
		return out, fmt.Errorf("teaser.text is required while the teaser is on")
	}
	if len(t.Paths) > maxTeaserPaths {
		return out, fmt.Errorf("teaser can list at most %d paths", maxTeaserPaths)
	}
	for _, p := range t.Paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		star := strings.Index(p, "*")
		if !strings.HasPrefix(p, "/") || len(p) > maxTeaserPath || strings.ContainsAny(p, " \t\n?#") || (star >= 0 && star != len(p)-1) {
			return out, fmt.Errorf("teaser path %q must start with / and may only end with *", p)
		}
		out.Paths = append(out.Paths, p)
	}
	return out, nil
}

// HomeItemTitle names a home item or help link; ok is false for IDs the widget does not show.
func HomeItemTitle(h models.ChatHome, id string) (string, bool) {
	for _, it := range h.Items {
		if it.ID == id {
			if it.Title == "" && it.Kind == models.ChatHomeItemMessage {
				return "Send us a message", true
			}
			return it.Title, true
		}
	}
	for _, l := range h.HelpLinks {
		if l.ID == id {
			return l.Title, true
		}
	}
	return "", false
}

// RecordClick refuses unknown IDs, so the counters cannot be filled with junk.
func (s *Service) RecordClick(w *models.ChatWidget, itemID string) bool {
	if !w.Home.Enabled || w.WorkspaceID == nil {
		return false
	}
	if _, ok := HomeItemTitle(w.Home, itemID); !ok {
		return false
	}
	return s.repo.AddClick(*w.WorkspaceID, w.ID, itemID, time.Now().UTC()) == nil
}
