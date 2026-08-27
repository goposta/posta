// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package dto

import "time"

// Response is the standard API response envelope with a generic data field.
type Response[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
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
	Key     string   `json:"key"`
	ID      uint     `json:"id"`
	Name    string   `json:"name"`
	Prefix  string   `json:"prefix"`
	Scopes  []string `json:"scopes" enum:"send,read,webhooks,*"`
	Message string   `json:"message"`
}

type MessageData struct {
	Message string `json:"message"`
}

type SMTPCredentialCreatedData struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message"`
}
