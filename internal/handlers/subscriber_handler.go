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
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
)

type SubscriberHandler struct {
	repo *repositories.SubscriberRepository
}

func NewSubscriberHandler(repo *repositories.SubscriberRepository) *SubscriberHandler {
	return &SubscriberHandler{repo: repo}
}

type CreateSubscriberRequest struct {
	Body struct {
		Email        string                 `json:"email" required:"true" format:"email"`
		Name         string                 `json:"name"`
		Status       string                 `json:"status"`
		CustomFields map[string]interface{} `json:"custom_fields"`
		Timezone     string                 `json:"timezone"`
		Language     string                 `json:"language"`
	} `json:"body"`
}

type UpdateSubscriberRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name         string                 `json:"name"`
		Status       string                 `json:"status"`
		CustomFields map[string]interface{} `json:"custom_fields"`
		Timezone     string                 `json:"timezone"`
		Language     string                 `json:"language"`
	} `json:"body"`
}

type DeleteSubscriberRequest struct {
	ID int `param:"id"`
}

type GetSubscriberRequest struct {
	ID int `param:"id"`
}

type ListSubscribersRequest struct {
	Page   int    `query:"page" default:"0"`
	Size   int    `query:"size" default:"20"`
	Search string `query:"search"`
	Status string `query:"status"`
}

type BulkImportSubscribersRequest struct {
	Body struct {
		Subscribers []BulkSubscriberEntry `json:"subscribers" required:"true"`
	} `json:"body"`
}

type BulkSubscriberEntry struct {
	Email        string                 `json:"email"`
	Name         string                 `json:"name"`
	Timezone     string                 `json:"timezone"`
	Language     string                 `json:"language"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

type BulkImportResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

func (h *SubscriberHandler) Create(c *okapi.Context, req *CreateSubscriberRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	scope := getScope(c)

	email := strings.ToLower(strings.TrimSpace(req.Body.Email))
	if email == "" {
		return c.AbortBadRequest("email is required")
	}

	status := models.SubscriberStatus(req.Body.Status)
	if status == "" {
		status = models.SubscriberStatusSubscribed
	}

	now := time.Now()
	s := &models.Subscriber{
		UserID:       scope.UserID,
		WorkspaceID:  scope.WorkspaceID,
		Email:        email,
		Name:         strings.TrimSpace(req.Body.Name),
		Status:       status,
		CustomFields: req.Body.CustomFields,
		Timezone:     strings.TrimSpace(req.Body.Timezone),
		Language:     strings.TrimSpace(req.Body.Language),
		SubscribedAt: &now,
	}

	if err := h.repo.Create(s); err != nil {
		return c.AbortConflict("subscriber already exists")
	}
	return created(c, s)
}

func (h *SubscriberHandler) List(c *okapi.Context, req *ListSubscribersRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)
	items, total, err := h.repo.FindByScope(getScope(c), req.Search, req.Status, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list subscribers")
	}
	return paginated(c, items, total, page, size)
}

func (h *SubscriberHandler) Get(c *okapi.Context, req *GetSubscriberRequest) error {
	s, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, s.UserID, s.WorkspaceID) {
		return c.AbortNotFound("subscriber not found")
	}
	return ok(c, s)
}

func (h *SubscriberHandler) Update(c *okapi.Context, req *UpdateSubscriberRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	s, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, s.UserID, s.WorkspaceID) {
		return c.AbortNotFound("subscriber not found")
	}

	if req.Body.Name != "" {
		s.Name = req.Body.Name
	}
	if req.Body.Status != "" {
		newStatus := models.SubscriberStatus(req.Body.Status)
		if newStatus == models.SubscriberStatusUnsubscribed && s.Status != models.SubscriberStatusUnsubscribed {
			now := time.Now()
			s.UnsubscribedAt = &now
		}
		s.Status = newStatus
	}
	if req.Body.CustomFields != nil {
		s.CustomFields = req.Body.CustomFields
	}
	if req.Body.Timezone != "" {
		s.Timezone = req.Body.Timezone
	}
	if req.Body.Language != "" {
		s.Language = req.Body.Language
	}
	now := time.Now()
	s.UpdatedAt = &now

	if err := h.repo.Update(s); err != nil {
		return c.AbortInternalServerError("failed to update subscriber")
	}
	return ok(c, s)
}

func (h *SubscriberHandler) Delete(c *okapi.Context, req *DeleteSubscriberRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	s, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, s.UserID, s.WorkspaceID) {
		return c.AbortNotFound("subscriber not found")
	}
	if err := h.repo.Delete(s.ID); err != nil {
		return c.AbortInternalServerError("failed to delete subscriber")
	}
	return noContent(c)
}

func (h *SubscriberHandler) BulkImportJSON(c *okapi.Context, req *BulkImportSubscribersRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	scope := getScope(c)

	now := time.Now()
	var subscribers []models.Subscriber
	for _, entry := range req.Body.Subscribers {
		email := strings.ToLower(strings.TrimSpace(entry.Email))
		if email == "" {
			continue
		}
		subscribers = append(subscribers, models.Subscriber{
			UserID:       scope.UserID,
			WorkspaceID:  scope.WorkspaceID,
			Email:        email,
			Name:         strings.TrimSpace(entry.Name),
			Timezone:     strings.TrimSpace(entry.Timezone),
			Language:     strings.TrimSpace(entry.Language),
			Status:       models.SubscriberStatusSubscribed,
			CustomFields: entry.CustomFields,
			SubscribedAt: &now,
		})
	}

	created, skipped, err := h.repo.BulkCreate(subscribers)
	if err != nil {
		return c.AbortInternalServerError("import failed: " + err.Error())
	}

	return ok(c, BulkImportResult{
		Created: created,
		Skipped: skipped,
		Total:   len(subscribers),
	})
}

func (h *SubscriberHandler) BulkImportCSV(c *okapi.Context) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	scope := getScope(c)

	file, _, err := c.Request().FormFile("file")
	if err != nil {
		return c.AbortBadRequest("file is required")
	}
	defer file.Close()

	// Parse optional column mapping: {"0":"email","1":"name","2":"custom_fields.company"}
	mappingStr := c.Request().FormValue("column_mapping")
	columnMap := map[int]string{0: "email", 1: "name"} // defaults
	if mappingStr != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(mappingStr), &raw); err == nil {
			columnMap = make(map[int]string)
			for k, v := range raw {
				idx, err := strconv.Atoi(k)
				if err != nil {
					continue
				}
				columnMap[idx] = v
			}
		}
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	// Skip header row
	if _, err := reader.Read(); err != nil {
		return c.AbortBadRequest("failed to read CSV header")
	}

	now := time.Now()
	var subscribers []models.Subscriber
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip bad rows
		}

		s := models.Subscriber{
			UserID:       scope.UserID,
			WorkspaceID:  scope.WorkspaceID,
			Status:       models.SubscriberStatusSubscribed,
			SubscribedAt: &now,
			CustomFields: make(models.CustomFields),
		}

		for idx, field := range columnMap {
			if idx >= len(record) {
				continue
			}
			val := strings.TrimSpace(record[idx])
			switch field {
			case "email":
				s.Email = strings.ToLower(val)
			case "name":
				s.Name = val
			case "status":
				s.Status = models.SubscriberStatus(val)
			case "language":
				s.Language = val
			default:
				if strings.HasPrefix(field, "custom_fields.") {
					key := strings.TrimPrefix(field, "custom_fields.")
					s.CustomFields[key] = val
				}
			}
		}

		if s.Email == "" {
			continue
		}
		subscribers = append(subscribers, s)
	}

	createdCount, skipped, err := h.repo.BulkCreate(subscribers)
	if err != nil {
		return c.AbortInternalServerError("import failed: " + err.Error())
	}

	return ok(c, BulkImportResult{
		Created: createdCount,
		Skipped: skipped,
		Total:   len(subscribers),
	})
}
