// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

type ActorRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func (ActorRef) TableName() string { return "users" }
