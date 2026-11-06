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

package models

import "time"

const (
	LDAPTLSNone     = "none"     // plain LDAP: development or a trusted LAN only
	LDAPTLSStartTLS = "starttls" // upgrade a plain connection with StartTLS
	LDAPTLSLDAPS    = "ldaps"    // implicit TLS, usually port 636
)

// AuthMethodLDAP marks directory-created and -managed accounts; they have no usable local password.
const AuthMethodLDAP = "ldap"

// The table exists in every build; only the Enterprise build reads it.
type LDAPConfig struct {
	ID uint `json:"id" gorm:"primaryKey"`
	// Name is the unique handle; DisplayName the label shown to admins.
	Name        string `json:"name" gorm:"uniqueIndex;not null"`
	DisplayName string `json:"display_name"`

	Host            string `json:"host" gorm:"not null"`
	Port            int    `json:"port" gorm:"not null;default:389"`
	TLSMode         string `json:"tls_mode" gorm:"not null;default:'starttls'"`
	CACertPEM       string `json:"ca_cert_pem" gorm:"type:text"`
	InsecureSkipTLS bool   `json:"insecure_skip_tls" gorm:"not null;default:false"`
	TimeoutSeconds  int    `json:"timeout_seconds" gorm:"not null;default:10"`

	// BindDN is the service account that finds the user entry before Posta binds as the user to check the password.
	BindDN string `json:"bind_dn"`
	// BindPasswordEnc is the service account's password, encrypted at rest.
	BindPasswordEnc string `json:"-" gorm:"type:text"`

	UserBaseDN string `json:"user_base_dn"`
	// UserFilter finds the user; %s is the escaped sign-in email.
	UserFilter string `json:"user_filter"`
	AttrEmail  string `json:"attr_email"` // blank: mail
	AttrName   string `json:"attr_name"`  // blank: displayName

	GroupBaseDN  string `json:"group_base_dn"`
	GroupFilter  string `json:"group_filter"`  // %s is the user's DN; blank reads MemberAttr
	MemberAttr   string `json:"member_attr"`   // blank: memberOf
	NestedGroups bool   `json:"nested_groups"` // Active Directory nested groups

	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Mappings []LDAPGroupMapping `json:"mappings,omitempty" gorm:"foreignKey:LDAPConfigID;constraint:OnDelete:CASCADE"`

	// BindPasswordSet says whether a service password is stored, without it.
	BindPasswordSet bool `json:"bind_password_set" gorm:"-"`
}

// Reconciled at every sign-in: PlatformAdmin grants admin, WorkspaceID with WorkspaceRole grants membership.
type LDAPGroupMapping struct {
	ID           uint `json:"id" gorm:"primaryKey"`
	LDAPConfigID uint `json:"ldap_config_id" gorm:"index;not null"`
	// GroupDN matches the user's groups case-insensitively; a bare CN such as "posta-admins" also works.
	GroupDN       string        `json:"group_dn" gorm:"not null"`
	PlatformAdmin bool          `json:"platform_admin" gorm:"not null;default:false"`
	WorkspaceID   *uint         `json:"workspace_id"`
	WorkspaceRole WorkspaceRole `json:"workspace_role"`
	CreatedAt     time.Time     `json:"created_at"`
}
