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

// New returns the Community stub: every Enterprise feature is refused and no
// license-verification code is linked. This is the only implementation
// compiled without the `enterprise` build tag.
func New(*gorm.DB, Options) EE { return community{} }
