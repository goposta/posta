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

// QuotaChecker verifies that creating a resource would not exceed plan limits.
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

func noContent(c *okapi.Context) error {
	return c.JSON(http.StatusNoContent, dto.Response[any]{
		Success: true,
	})
}

// getScope extracts the ResourceScope from the request context.
// If workspace_id is set (via middleware), targets that workspace; otherwise personal.
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

// ownsResource checks whether a resource belongs to the current scope.
func ownsResource(c *okapi.Context, resourceUserID uint, resourceWorkspaceID *uint) bool {
	return repositories.OwnsResource(getScope(c), resourceUserID, resourceWorkspaceID)
}

// workspaceRole returns the workspace role from context, or empty string if personal mode.
func workspaceRole(c *okapi.Context) models.WorkspaceRole {
	return models.WorkspaceRole(c.GetString("workspace_role"))
}

// canEditInWorkspace returns true if the user can create/modify resources.
// In personal mode, always allowed. In workspace mode, requires Editor+ role.
func canEditInWorkspace(c *okapi.Context) bool {
	role := workspaceRole(c)
	if role == "" {
		return true // personal mode
	}
	return role.CanEdit()
}

// requireEdit returns errForbidden when the user lacks editor+ permission.
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
