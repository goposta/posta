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

package dto

// Response is the standard API response envelope with a generic data field.
type Response[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data,omitempty"`
}

// PageableResponse is the paginated API response envelope.
type PageableResponse[T any] struct {
	Success  bool     `json:"success"`
	Data     []T      `json:"data"`
	Pageable Pageable `json:"pageable"`
}

// Pageable holds pagination metadata.
type Pageable struct {
	CurrentPage   int   `json:"current_page"`
	Size          int   `json:"size"`
	TotalPages    int   `json:"total_pages"`
	TotalElements int64 `json:"total_elements"`
	Empty         bool  `json:"empty"`
}

// ErrorResponseBody is the error envelope returned by the custom error handler.
type ErrorResponseBody struct {
	Success bool       `json:"success"`
	Data    any        `json:"data"`
	Error   *ErrorInfo `json:"error"`
}

// ErrorInfo holds error details.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

type APIKeyCreatedData struct {
	Key     string `json:"key"`
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Prefix  string `json:"prefix"`
	Message string `json:"message"`
}

type MessageData struct {
	Message string `json:"message"`
}
