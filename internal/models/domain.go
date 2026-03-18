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

package models

import "time"

type Domain struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	UserID            uint      `json:"user_id" gorm:"uniqueIndex:idx_user_domain;not null"`
	Domain            string    `json:"domain" gorm:"uniqueIndex:idx_user_domain;not null"`
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
