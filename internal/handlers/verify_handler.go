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
	"errors"

	"github.com/goposta/posta/internal/services/verifier"
	"github.com/jkaninda/okapi"
)

// VerifyHandler exposes the email-verification endpoint.
type VerifyHandler struct {
	svc *verifier.Service
}

func NewVerifyHandler(svc *verifier.Service) *VerifyHandler {
	return &VerifyHandler{svc: svc}
}

// VerifyAddressRequest is the body for POST /emails/verify. The email is
// validated with format:"email", so a syntactically malformed address is
// rejected with a 400 before the handler runs; the verifier still re-checks
// syntax as a safety net.
type VerifyAddressRequest struct {
	Fresh bool `query:"fresh" doc:"Bypass the cache and re-check the address"`
	Body  struct {
		Email string `json:"email" required:"true" format:"email" doc:"Email address to verify"`
	} `json:"body"`
}

// Verify checks whether an email address is valid/deliverable.
func (h *VerifyHandler) Verify(c *okapi.Context, req *VerifyAddressRequest) error {
	if h.svc == nil || !h.svc.Enabled() {
		return c.AbortNotFound("email verification is disabled")
	}

	res, err := h.svc.Verify(c.Request().Context(), getScope(c), req.Body.Email, req.Fresh)
	if err != nil {
		if errors.Is(err, verifier.ErrRateLimited) {
			return c.AbortTooManyRequests("email verification rate limit exceeded")
		}
		return c.AbortInternalServerError("failed to verify email")
	}
	return ok(c, res)
}
