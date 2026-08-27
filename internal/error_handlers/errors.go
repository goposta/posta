// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package errorhandlers

import (
	"github.com/goposta/posta/internal/dto"
	"github.com/jkaninda/okapi"
)

// CustomErrorHandler returns an okapi.ErrorHandler that formats errors
func CustomErrorHandler() okapi.ErrorHandler {
	return func(c *okapi.Context, code int, message string, err error) error {
		return c.JSON(code, dto.ErrorResponseBody{
			Success: false,
			Data:    nil,
			Error: &dto.ErrorInfo{
				Code:    httpStatusToCode(code),
				Error:   err.Error(),
				Message: message,
			},
		})
	}
}

func httpStatusToCode(status int) string {
	switch status {
	case 400:
		return "BAD_REQUEST"
	case 401:
		return "UNAUTHORIZED"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 405:
		return "METHOD_NOT_ALLOWED"
	case 409:
		return "CONFLICT"
	case 422:
		return "UNPROCESSABLE_ENTITY"
	case 429:
		return "TOO_MANY_REQUESTS"
	case 500:
		return "INTERNAL_SERVER_ERROR"
	case 502:
		return "BAD_GATEWAY"
	case 503:
		return "SERVICE_UNAVAILABLE"
	default:
		return "ERROR"
	}
}
