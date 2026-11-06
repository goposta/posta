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

package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/chat"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runs only when POSTA_CHAT_TEST_DSN is set, with no default, so it can never touch a dev database by accident.
func TestChatRetentionAgainstDB(t *testing.T) {
	dsn := os.Getenv("POSTA_CHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTA_CHAT_TEST_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var ws models.Workspace
	if err := db.First(&ws).Error; err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewChatRepository(db)
	key, _ := models.NewChatWidgetKey()
	w := &models.ChatWidget{WorkspaceID: &ws.ID, Name: "retention-" + key[4:12], DisplayName: "Retention", PublicKey: key,
		Status: models.ChatWidgetStatusActive, RetentionDays: 30}
	if err := repo.CreateWidget(w); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	old, recent := now.AddDate(0, 0, -45), now.AddDate(0, 0, -5)
	mk := func(state models.ChatState, last time.Time, ip string) *models.ChatConversation {
		_, hash, _ := chat.NewVisitorToken()
		v := &models.ChatVisitor{WorkspaceID: &ws.ID, WidgetID: w.ID, TokenHash: hash, FirstSeenIP: ip, LastSeenIP: ip, LastSeenAt: last}
		if err := repo.CreateVisitor(v); err != nil {
			t.Fatal(err)
		}
		c := &models.ChatConversation{WorkspaceID: &ws.ID, WidgetID: w.ID, VisitorID: v.ID, State: state, Status: models.MessageStatusReceived}
		if err := repo.CreateConversation(c, &models.ChatMessage{AuthorKind: models.ChatAuthorVisitor, Body: "hi"}); err != nil {
			t.Fatal(err)
		}
		db.Model(c).Update("last_message_at", last)
		return c
	}
	expired := mk(models.ChatStateClosed, old, "203.0.113.77")
	openOld := mk(models.ChatStateOpen, old, "198.51.100.9")
	closedRecent := mk(models.ChatStateClosed, recent, "192.0.2.10")

	job := NewChatRetentionJob(chat.NewService(repo, nil, nil, nil, nil, chat.DefaultConfig()), 365)
	if err := job.Run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	gone := func(c *models.ChatConversation) bool {
		var n int64
		db.Unscoped().Model(&models.ChatConversation{}).Where("id = ?", c.ID).Count(&n)
		return n == 0
	}
	if !gone(expired) {
		t.Error("a closed conversation past the widget's 30 days was kept")
	}
	if gone(openOld) || gone(closedRecent) {
		t.Error("an open or recent conversation was erased")
	}
	var msgs int64
	db.Model(&models.ChatMessage{}).Where("conversation_id = ?", expired.ID).Count(&msgs)
	var visitors int64
	db.Model(&models.ChatVisitor{}).Where("id = ?", expired.VisitorID).Count(&visitors)
	if msgs != 0 || visitors != 0 {
		t.Errorf("left %d messages and %d visitors of the erased conversation", msgs, visitors)
	}
	var v models.ChatVisitor
	db.First(&v, openOld.VisitorID)
	if v.FirstSeenIP != "198.51.100.0" || v.LastSeenIP != "198.51.100.0" {
		t.Errorf("old visitor's IP not truncated: %q/%q", v.FirstSeenIP, v.LastSeenIP)
	}
	var recentVisitor models.ChatVisitor
	db.First(&recentVisitor, closedRecent.VisitorID)
	if recentVisitor.FirstSeenIP != "192.0.2.10" {
		t.Errorf("a recent visitor's IP was truncated: %q", recentVisitor.FirstSeenIP)
	}
	_, _ = repo.EraseConversation(openOld.ID)
	_, _ = repo.EraseConversation(closedRecent.ID)
	db.Unscoped().Delete(w)
}
