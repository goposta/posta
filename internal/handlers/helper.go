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

package handlers

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

// planSourceGlobalSettings stands in for a missing plan; the dashboard branches on this exact value.
const planSourceGlobalSettings = "global_settings"

var errForbidden = errors.New("insufficient workspace permissions: editor role or higher required")

// 1x1 transparent GIF
var transparentPixel, _ = base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")
var botUASubstrings = []string{
	"bot", "crawler", "spider", "scanner",
	"barracuda", "mimecast", "proofpoint",
	"symantec", "trendmicro", "fortinet",
	"sophos", "messagelabs", "ironport",
	"safelinks",
	"googleimageproxy",
	"yahoomailproxy",
	"http-client", "go-http-client", "curl/", "wget/",
	"headlesschrome", "phantomjs", "puppeteer", "playwright",
}

type QuotaChecker interface {
	CheckQuota(db *gorm.DB, userID uint, workspaceID *uint, resource string) error
	CheckWorkspaceQuota(db *gorm.DB, userID uint) error
}

func ok[T any](c *okapi.Context, data T) error {
	return c.JSON(http.StatusOK, dto.Response[T]{
		Success: true,
		Data:    data,
	})
}

func created[T any](c *okapi.Context, data T) error {
	return c.JSON(http.StatusCreated, dto.Response[T]{
		Success: true,
		Data:    data,
	})
}

// noContent answers 204 with no body, as HTTP requires for that status.
func noContent(c *okapi.Context) error {
	return c.NoContent()
}

// getScope targets the middleware-set workspace_id, or the personal scope when there is none.
func getScope(c *okapi.Context) repositories.ResourceScope {
	userID := uint(c.GetInt("user_id"))
	scope := repositories.ResourceScope{UserID: userID}
	wsID := c.GetInt("workspace_id")
	if wsID > 0 {
		wid := uint(wsID)
		scope.WorkspaceID = &wid
	}
	return scope
}

func ownsResource(c *okapi.Context, resourceUserID uint, resourceWorkspaceID *uint) bool {
	return repositories.OwnsResource(getScope(c), resourceUserID, resourceWorkspaceID)
}

func workspaceRole(c *okapi.Context) models.WorkspaceRole {
	return models.WorkspaceRole(c.GetString("workspace_role"))
}

// Personal mode always allows; workspace mode requires the Editor role or above.
func canEditInWorkspace(c *okapi.Context) bool {
	role := workspaceRole(c)
	if role == "" {
		return true // personal mode
	}
	return role.CanEdit()
}

func requireEdit(c *okapi.Context) error {
	if !canEditInWorkspace(c) {
		return errForbidden
	}
	return nil
}

func paginated[T any](c *okapi.Context, items []T, total int64, page, size int) error {
	if items == nil {
		items = []T{}
	}
	totalPages := 0
	if size > 0 {
		totalPages = int((total + int64(size) - 1) / int64(size))
	}
	return c.JSON(http.StatusOK, dto.PageableResponse[T]{
		Success: true,
		Data:    items,
		Pageable: dto.Pageable{
			CurrentPage:   page,
			Size:          size,
			TotalPages:    totalPages,
			TotalElements: total,
			Empty:         len(items) == 0,
		},
	})
}
func isBotUA(ua string) bool {
	if ua == "" {
		return true
	}
	low := strings.ToLower(ua)
	for _, needle := range botUASubstrings {
		if strings.Contains(low, needle) {
			return true
		}
	}
	return false
}
