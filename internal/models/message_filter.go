// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"time"

	"github.com/lib/pq"
)

type FilterKind string

const (
	FilterKindKeyword FilterKind = "keyword"
	FilterKindPhrase  FilterKind = "phrase"
	FilterKindRegex   FilterKind = "regex"
	FilterKindEmail   FilterKind = "email"
	FilterKindDomain  FilterKind = "domain"
	FilterKindIP      FilterKind = "ip"
)

var ValidFilterKinds = map[FilterKind]bool{
	FilterKindKeyword: true,
	FilterKindPhrase:  true,
	FilterKindRegex:   true,
	FilterKindEmail:   true,
	FilterKindDomain:  true,
	FilterKindIP:      true,
}

type FilterAction string

const (
	FilterActionScore      FilterAction = "score"
	FilterActionFlag       FilterAction = "flag"
	FilterActionQuarantine FilterAction = "quarantine"
	FilterActionReject     FilterAction = "reject"
	FilterActionAllowlist  FilterAction = "allowlist"
)

var ValidFilterActions = map[FilterAction]bool{
	FilterActionScore:      true,
	FilterActionFlag:       true,
	FilterActionQuarantine: true,
	FilterActionReject:     true,
	FilterActionAllowlist:  true,
}

const MaxFilterPatternLength = 512

type MessageFilter struct {
	ID          uint  `json:"id" gorm:"primaryKey"`
	WorkspaceID *uint `json:"workspace_id,omitempty" gorm:"index;not null"`
	FormID      *uint `json:"form_id,omitempty" gorm:"index"`

	Kind          FilterKind     `json:"kind" gorm:"type:varchar(16);not null"`
	Pattern       string         `json:"pattern" gorm:"not null"`
	Action        FilterAction   `json:"action" gorm:"type:varchar(16);default:'score';not null"`
	Score         float64        `json:"score" gorm:"default:3;not null"`
	Fields        pq.StringArray `json:"fields" gorm:"type:text[]"`
	CaseSensitive bool           `json:"case_sensitive" gorm:"default:false;not null"`
	Enabled       bool           `json:"enabled" gorm:"default:true;index;not null"`

	HitCount  int64      `json:"hit_count" gorm:"default:0;not null"`
	LastHitAt *time.Time `json:"last_hit_at,omitempty"`
	Note      string     `json:"note"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}
