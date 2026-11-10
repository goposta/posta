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
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/chat"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

type ChatWidgetAssetRequest struct {
	ID    int    `param:"id"`
	Asset string `param:"asset"`
}

type ChatPublicAssetRequest struct {
	ID string `param:"id"`
}

// SetService enables widget image uploads, which share the chat service's blob store.
func (h *ChatWidgetHandler) SetService(svc *chat.Service) { h.svc = svc }

// UploadAsset takes a multipart file part: a PNG, JPEG, GIF or WebP image up to 512 KB.
func (h *ChatWidgetHandler) UploadAsset(c *okapi.Context) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(id))
	if err != nil || h.svc == nil {
		return c.AbortNotFound("chat widget not found")
	}
	if err := h.ee().RequireMutable(); err != nil {
		return licenseAbort(c, err)
	}
	r := c.Request()
	r.Body = http.MaxBytesReader(c.ResponseWriter(), r.Body, chat.MaxWidgetAssetBytes+64<<10)
	if err := r.ParseMultipartForm(chat.MaxWidgetAssetBytes + 64<<10); err != nil {
		return c.AbortRequestEntityTooLarge(chat.ErrAssetTooLarge.Error())
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return c.AbortBadRequest("a file part is required")
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, chat.MaxWidgetAssetBytes+1))
	if err != nil {
		return c.AbortBadRequest("failed to read the file")
	}
	a, err := h.svc.StoreWidgetAsset(r.Context(), w, header.Filename, data)
	switch {
	case errors.Is(err, chat.ErrAssetTooLarge):
		return c.AbortRequestEntityTooLarge(err.Error())
	case errors.Is(err, chat.ErrAssetType):
		return c.AbortBadRequest(err.Error())
	case err != nil:
		logger.Error("chat: failed to store widget image", "widget_id", w.ID, "error", err)
		return c.AbortInternalServerError("failed to store the image")
	}
	h.log(c, "chat_widget.asset_uploaded", fmt.Sprintf("Image %s uploaded to chat widget %s", a.Filename, w.DisplayName),
		map[string]any{metaChatWidgetID: w.ID})
	return created(c, a)
}

func (h *ChatWidgetHandler) ListAssets(c *okapi.Context, req *ChatWidgetIDRequest) error {
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("chat widget not found")
	}
	items, err := h.repo.ListWidgetAssets(w.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to list images")
	}
	for i := range items {
		items[i].URL = chat.AssetPath(items[i].UUID)
	}
	return ok(c, items)
}

// DeleteAsset refuses an image the home screen still shows, so the widget never renders a broken one.
func (h *ChatWidgetHandler) DeleteAsset(c *okapi.Context, req *ChatWidgetAssetRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(req.ID))
	if err != nil || h.svc == nil {
		return c.AbortNotFound("chat widget not found")
	}
	a, err := h.repo.FindWidgetAsset(w.ID, req.Asset)
	if err != nil {
		return c.AbortNotFound("image not found")
	}
	if slices.Contains(w.Home.AssetIDs(), a.UUID) {
		return c.AbortConflict("the home screen still shows this image; remove it there first")
	}
	if err := h.svc.DeleteWidgetAsset(c.Request().Context(), a); err != nil {
		return c.AbortInternalServerError("failed to delete the image")
	}
	h.log(c, "chat_widget.asset_deleted", fmt.Sprintf("Image %s deleted from chat widget %s", a.Filename, w.DisplayName),
		map[string]any{metaChatWidgetID: w.ID})
	return noContent(c)
}

// checkAssets makes sure the home screen names only this widget's own images.
func (h *ChatWidgetHandler) checkAssets(w *models.ChatWidget) error {
	ids := slices.Compact(slices.Sorted(slices.Values(w.Home.AssetIDs())))
	if len(ids) == 0 {
		return nil
	}
	n, err := h.repo.CountWidgetAssets(w.ID, ids)
	if err != nil || int(n) != len(ids) {
		return fmt.Errorf("the home screen names an image that is not uploaded to this widget")
	}
	return nil
}

// Asset serves a widget image to any page: new uploads get new IDs, so the response is cached for good.
func (h *ChatVisitorHandler) Asset(c *okapi.Context, req *ChatPublicAssetRequest) error {
	a, err := h.repo.FindPublicAsset(req.ID)
	if err != nil {
		return c.AbortNotFound("image not found")
	}
	body, err := h.svc.OpenWidgetAsset(c.Request().Context(), a)
	if err != nil {
		return c.AbortNotFound("image not found")
	}
	defer func() { _ = body.Close() }()
	hdr := c.ResponseWriter().Header()
	hdr.Set("Content-Type", a.ContentType)
	hdr.Set("Cache-Control", "public, max-age=31536000, immutable")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Cross-Origin-Resource-Policy", "cross-origin")
	hdr.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.ResponseWriter(), body)
	return err
}
