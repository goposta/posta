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

// InstallIDKey is the settings row holding the Install ID. The "app." prefix
// keeps it out of the admin settings page.
const InstallIDKey = "app.install_id"

// EnsureInstallID returns this deployment's Install ID, creating it on first
// call. A license can be bound to it. Community has one too, so an admin can
// copy it when buying a license. A restored copy of the database carries the
// same Install ID; a fresh installation gets a new one.
func EnsureInstallID(db *gorm.DB) (string, error) {
	row := models.Setting{Key: InstallIDKey}
	err := db.Where(models.Setting{Key: InstallIDKey}).
		Attrs(models.Setting{Value: "pst_" + strings.ReplaceAll(uuid.NewString(), "-", ""), Type: models.SettingTypeString}).
		FirstOrCreate(&row).Error
	return row.Value, err
}
