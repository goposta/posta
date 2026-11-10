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

package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
)

const MaxWidgetAssetBytes = 512 << 10

var (
	ErrAssetTooLarge = errors.New("images can be at most 512 KB")
	ErrAssetType     = errors.New("only PNG, JPEG, GIF and WebP images are accepted")
)

// SVG is left out on purpose: it can carry script.
var widgetAssetTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// AssetPath is the public path of an asset; the widget resolves it against its own script origin.
func AssetPath(id string) string {
	if id == "" {
		return ""
	}
	return "/widget/v1/assets/" + id
}

// StoreWidgetAsset keeps a logo working on installs without blob storage by falling back to the database.
func (s *Service) StoreWidgetAsset(ctx context.Context, w *models.ChatWidget, filename string, data []byte) (*models.ChatWidgetAsset, error) {
	if len(data) == 0 || len(data) > MaxWidgetAssetBytes {
		return nil, ErrAssetTooLarge
	}
	contentType := http.DetectContentType(data)
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	ext, ok := widgetAssetTypes[contentType]
	if !ok {
		return nil, ErrAssetType
	}
	name := CleanLine(path.Base(strings.ReplaceAll(filename, "\\", "/")), 120)
	if name == "" || name == "." || name == "/" {
		name = "image" + ext
	}
	a := &models.ChatWidgetAsset{
		UUID: uuid.NewString(), WorkspaceID: w.WorkspaceID, WidgetID: w.ID,
		Filename: name, ContentType: contentType, Size: int64(len(data)),
	}
	if s.blobs != nil {
		a.Key = fmt.Sprintf("chat-assets/%d/%d/%s%s", *w.WorkspaceID, w.ID, a.UUID, ext)
		if err := s.blobs.Put(ctx, a.Key, bytes.NewReader(data), contentType); err != nil {
			return nil, err
		}
	} else {
		a.Data = data
	}
	if err := s.repo.CreateWidgetAsset(a); err != nil {
		if a.Key != "" {
			_ = s.blobs.Delete(ctx, a.Key)
		}
		return nil, err
	}
	a.URL = AssetPath(a.UUID)
	return a, nil
}

func (s *Service) OpenWidgetAsset(ctx context.Context, a *models.ChatWidgetAsset) (io.ReadCloser, error) {
	if a.Key == "" {
		return io.NopCloser(bytes.NewReader(a.Data)), nil
	}
	if s.blobs == nil {
		return nil, ErrAttachmentsOff
	}
	return s.blobs.Get(ctx, a.Key)
}

func (s *Service) DeleteWidgetAsset(ctx context.Context, a *models.ChatWidgetAsset) error {
	if err := s.repo.DeleteWidgetAsset(a.ID); err != nil {
		return err
	}
	if a.Key != "" && s.blobs != nil {
		_ = s.blobs.Delete(ctx, a.Key)
	}
	return nil
}

// PublicHome is the home screen as the widget receives it, with asset IDs resolved to paths.
type PublicHome struct {
	models.ChatHome
	LogoURL string           `json:"logo_url,omitempty"`
	Items   []PublicHomeItem `json:"items,omitempty"`
}

type PublicHomeItem struct {
	models.ChatHomeItem
	ImageURL string `json:"image_url,omitempty"`
}

func ToPublicHome(h models.ChatHome) *PublicHome {
	out := &PublicHome{ChatHome: h, LogoURL: AssetPath(h.LogoAsset)}
	for _, it := range h.Items {
		out.Items = append(out.Items, PublicHomeItem{ChatHomeItem: it, ImageURL: AssetPath(it.ImageAsset)})
	}
	return out
}
