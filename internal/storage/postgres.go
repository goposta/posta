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

package storage

import (
	"fmt"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ConnectPostgres opens a connection to PostgreSQL without running migrations.
func ConnectPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	logger.Info("database connected")
	return db, nil
}

// SeedAdmin calls approve only once an admin will really be created, so a changed in-app password never blocks boot.
func SeedAdmin(db *gorm.DB, email, password string, approve func() error) error {
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count > 0 {
		return nil
	}

	if approve != nil {
		if err := approve(); err != nil {
			return err
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	now := time.Now()
	admin := &models.User{
		Email:           email,
		Name:            "Admin",
		PasswordHash:    string(hash),
		Role:            models.UserRoleAdmin,
		EmailVerifiedAt: &now,
	}
	if err := db.Create(admin).Error; err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	logger.Info("default admin user created", "email", email)
	return nil
}
