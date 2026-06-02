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
