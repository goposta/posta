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
	"net/http"

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/dto"
)

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

func paginated[T any](c *okapi.Context, items []T, total int64, page, size int) error {
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
