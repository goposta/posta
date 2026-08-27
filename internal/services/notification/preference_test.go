// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package notification

import (
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestTemplateEnabled(t *testing.T) {
	off := &models.UserSetting{
		DailyReport:             false,
		NotifyBounceAlerts:      false,
		NotifyAPIKeyExpiry:      false,
		NotifyWorkspaceActivity: false,
	}
	on := &models.UserSetting{
		DailyReport:             true,
		NotifyBounceAlerts:      true,
		NotifyAPIKeyExpiry:      true,
		NotifyWorkspaceActivity: true,
	}

	gated := map[string]struct{ enabledFlag bool }{
		TemplateDailyReport:  {},
		TemplateBounceAlert:  {},
		TemplateAPIKeyExpiry: {},
		TemplateRoleChanged:  {},
	}
	for tmpl := range gated {
		if templateEnabled(tmpl, off) {
			t.Errorf("%s: want disabled when its flag is off", tmpl)
		}
		if !templateEnabled(tmpl, on) {
			t.Errorf("%s: want enabled when its flag is on", tmpl)
		}
	}

	for _, tmpl := range []string{
		TemplateLoginAlert, TemplateTwoFactorChange, TemplateAccountDeletion,
		TemplatePasswordChanged, TemplateWelcome, TemplateEmailVerify,
	} {
		if !templateEnabled(tmpl, off) {
			t.Errorf("%s: must always be allowed (not user-toggleable)", tmpl)
		}
	}

	if !templateEnabled("does_not_exist", off) {
		t.Error("unknown template should default to allowed")
	}
}
