// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"github.com/goposta/posta/internal/config"
	"github.com/jkaninda/okapi"
)

type AppInfo struct {
	Name        string `json:"name" example:"Posta"`
	Version     string `json:"version" example:"v1.0.0"`
	CommitID    string `json:"commit_id" example:"abc1234"`
	OpenAPIDocs bool   `json:"openapi_docs" example:"true"`
}

func Info(c *okapi.Context) error {
	cfg := config.New()
	return ok(c, AppInfo{
		Name:        "Posta",
		Version:     config.Version,
		CommitID:    config.CommitID,
		OpenAPIDocs: cfg.OpenAPIDocs,
	})
}
