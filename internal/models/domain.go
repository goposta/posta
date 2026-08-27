// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type Domain struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	UserID            uint      `json:"user_id" gorm:"index;not null"`
	WorkspaceID       *uint     `json:"workspace_id,omitempty" gorm:"index"`
	Domain            string    `json:"domain" gorm:"not null"`
	OwnershipVerified bool      `json:"ownership_verified" gorm:"default:false"`
	SPFVerified       bool      `json:"spf_verified" gorm:"default:false"`
	DKIMVerified      bool      `json:"dkim_verified" gorm:"default:false"`
	DMARCVerified     bool      `json:"dmarc_verified" gorm:"default:false"`
	VerificationToken string    `json:"verification_token" gorm:"not null"`
	CreatedAt         time.Time `json:"created_at"`

	User User `json:"-" gorm:"foreignKey:UserID"`
}

// IsOwnershipVerified returns true when domain ownership has been confirmed via TXT record.
func (d *Domain) IsOwnershipVerified() bool {
	return d.OwnershipVerified
}

// IsFullyVerified returns true when ownership is confirmed and all DNS checks pass.
func (d *Domain) IsFullyVerified() bool {
	return d.OwnershipVerified && d.SPFVerified && d.DKIMVerified && d.DMARCVerified
}
