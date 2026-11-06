// Copyright (c) 2026 Jonas Kaninda. All rights reserved.
//
// This file is part of Posta Enterprise Edition.
//
// This file is licensed under the Posta Enterprise License, not the GNU
// Affero General Public License. See internal/enterprise/LICENSE.md.
//
// SPDX-License-Identifier: LicenseRef-Posta-Enterprise

package enterprise

import (
	"strings"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

// The "app." prefix keeps the Install ID out of the admin settings page.
const InstallIDKey = "app.install_id"

// Community has one too so admins can copy it when buying; a restored database keeps the same Install ID.
func EnsureInstallID(db *gorm.DB) (string, error) {
	row := models.Setting{Key: InstallIDKey}
	err := db.Where(models.Setting{Key: InstallIDKey}).
		Attrs(models.Setting{Value: "pst_" + strings.ReplaceAll(uuid.NewString(), "-", ""), Type: models.SettingTypeString}).
		FirstOrCreate(&row).Error
	return row.Value, err
}
