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

package seeder

import (
	"strings"
	"testing"

	"github.com/goposta/posta/internal/services/email"
)

// TestDefaultTemplatesRender renders every seed template with the strict missingkey=error renderer of the send path.
func TestDefaultTemplatesRender(t *testing.T) {
	defs := defaultTemplateDefs("Jonas", 2026, "https://example.com/docs")
	if len(defs) == 0 {
		t.Fatal("no default templates defined")
	}

	renderer := email.NewTemplateRenderer() // defaults to missingkey=error

	for _, def := range defs {
		t.Run(def.DisplayName, func(t *testing.T) {
			for _, lang := range seedLanguages {
				subject, ok := def.Subjects[lang]
				if !ok || strings.TrimSpace(subject) == "" {
					t.Fatalf("%s/%s: missing subject", def.DisplayName, lang)
				}

				// WithSystemVars injects the reserved posta_* sentinels the same way the send path does.
				data := email.WithSystemVars(def.SampleData)

				out, err := renderer.Render(&email.RenderInput{
					SubjectTemplate: subject,
					HTMLTemplate:    htmlTmpl(def.Base, lang),
					TextTemplate:    textTmpl(def.Base, lang),
					CSS:             defaultCSS,
				}, data)
				if err != nil {
					t.Fatalf("%s/%s: render failed: %v", def.DisplayName, lang, err)
				}

				if strings.TrimSpace(out.Subject) == "" {
					t.Errorf("%s/%s: empty subject", def.DisplayName, lang)
				}
				if strings.TrimSpace(out.HTML) == "" {
					t.Errorf("%s/%s: empty HTML", def.DisplayName, lang)
				}
				if strings.TrimSpace(out.Text) == "" {
					t.Errorf("%s/%s: empty text", def.DisplayName, lang)
				}
				// Every seeded template links the hosted web view, so a system sentinel must survive into the HTML.
				if !email.HasSystemSentinels(out.HTML) {
					t.Errorf("%s/%s: expected a posta_* system link in rendered HTML", def.DisplayName, lang)
				}
			}
		})
	}
}
