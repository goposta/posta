//go:build !enterprise

// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

package enterprise

import "gorm.io/gorm"

// New is the only implementation without the enterprise tag: it refuses everything and links no verifier.
func New(*gorm.DB, Options) EE { return community{} }
