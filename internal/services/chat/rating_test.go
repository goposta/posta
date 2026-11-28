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
	"context"
	"errors"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestRateRefusesBeforeStoring(t *testing.T) {
	s := &Service{}
	asking := &models.ChatWidget{AskRating: true}
	closed := &models.ChatConversation{State: models.ChatStateClosed}
	cases := []struct {
		name   string
		widget *models.ChatWidget
		conv   *models.ChatConversation
		rating string
		want   error
	}{
		{"widget does not ask", &models.ChatWidget{}, closed, models.ChatRatingGood, ErrRatingOff},
		{"unknown rating", asking, closed, "great", ErrRatingInvalid},
		{"still open", asking, &models.ChatConversation{State: models.ChatStateOpen}, models.ChatRatingBad, ErrRatingClosed},
		{"pending", asking, &models.ChatConversation{State: models.ChatStatePending}, models.ChatRatingGood, ErrRatingClosed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := s.Rate(context.Background(), c.widget, c.conv, c.rating, ""); !errors.Is(err, c.want) {
				t.Fatalf("Rate() = %v, want %v", err, c.want)
			}
		})
	}
}
