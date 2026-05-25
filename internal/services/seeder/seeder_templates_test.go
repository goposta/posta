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

package seeder

import (
	"strings"
	"testing"

	"github.com/goposta/posta/internal/services/email"
)

// TestDefaultTemplatesRender renders every seeded template (subject, HTML, text;
// EN and FR) with its own sample data using the strict missingkey=error renderer
// that the real send path uses. This catches template parse errors and any
// referenced variable that the sample data forgot to provide.
func TestDefaultTemplatesRender(t *testing.T) {
	defs := defaultTemplateDefs("Jonas", 2026, "https://example.com/docs")
	if len(defs) == 0 {
		t.Fatal("no default templates defined")
	}

	renderer := email.NewTemplateRenderer() // defaults to missingkey=error

	for _, def := range defs {
		t.Run(def.Name, func(t *testing.T) {
			for _, lang := range seedLanguages {
				subject, ok := def.Subjects[lang]
				if !ok || strings.TrimSpace(subject) == "" {
					t.Fatalf("%s/%s: missing subject", def.Name, lang)
				}

				// WithSystemVars injects the reserved posta_* variables (as
				// sentinels) the same way the send path does.
				data := email.WithSystemVars(def.SampleData)

				out, err := renderer.Render(&email.RenderInput{
					SubjectTemplate: subject,
					HTMLTemplate:    htmlTmpl(def.Base, lang),
					TextTemplate:    textTmpl(def.Base, lang),
					CSS:             defaultCSS,
				}, data)
				if err != nil {
					t.Fatalf("%s/%s: render failed: %v", def.Name, lang, err)
				}

				if strings.TrimSpace(out.Subject) == "" {
					t.Errorf("%s/%s: empty subject", def.Name, lang)
				}
				if strings.TrimSpace(out.HTML) == "" {
					t.Errorf("%s/%s: empty HTML", def.Name, lang)
				}
				if strings.TrimSpace(out.Text) == "" {
					t.Errorf("%s/%s: empty text", def.Name, lang)
				}
				// Every seeded template links the hosted web view, so a reserved
				// system sentinel must survive into the rendered HTML.
				if !email.HasSystemSentinels(out.HTML) {
					t.Errorf("%s/%s: expected a posta_* system link in rendered HTML", def.Name, lang)
				}
			}
		})
	}
}
