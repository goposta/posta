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
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

type formDef struct {
	DisplayName  string
	Name         string
	Description  string
	Mode         models.FormMode
	Confirmation string
	Fields       models.FieldDefs
}

// defaultFormDefs returns the example forms seeded into every new workspace:
// a contact form whose submissions are conversations, a survey whose
// responses are read as data, and a form over several pages. Kept as a pure
// function (no DB) so tests can check each schema the way the API would.
func defaultFormDefs() []formDef {
	return []formDef{
		{
			DisplayName:  "Contact form",
			Name:         "contact",
			Description:  "Questions and requests from your website, answered from Messages.",
			Mode:         models.FormModeInbox,
			Confirmation: "Thanks for getting in touch. We'll reply by email soon.",
			Fields: models.FieldDefs{
				{Key: "name", Label: "Name", Type: models.FieldText, Required: true, Role: models.RoleSenderName, PII: true, MaxLen: intPtr(120)},
				senderEmail(),
				{Key: "subject", Label: "Subject", Type: models.FieldText, Role: models.RoleSubject, MaxLen: intPtr(200)},
				{Key: "message", Label: "Message", Type: models.FieldTextarea, Required: true, Role: models.RoleBody, MinLen: intPtr(10), MaxLen: intPtr(5000)},
			},
		},
		{
			DisplayName:  "Customer feedback",
			Name:         "customer-feedback",
			Description:  "A short survey. Its responses are summarised per question and can be exported.",
			Mode:         models.FormModeCollect,
			Confirmation: "Thank you, your feedback helps us improve.",
			Fields: models.FieldDefs{
				{Key: "satisfaction", Label: "How satisfied are you overall?", Type: models.FieldRating, Required: true, Max: floatPtr(5)},
				{
					Key: "recommend", Label: "How likely are you to recommend us to a friend?", Type: models.FieldScale, Required: true,
					Min: floatPtr(0), Max: floatPtr(10), MinLabel: "Not likely", MaxLabel: "Very likely",
				},
				{
					Key: "used_for", Label: "What do you use us for?", Type: models.FieldMultiSelect, Display: models.DisplayCheckboxes,
					Options: []models.FieldOption{
						{Value: "work", Label: "Work"},
						{Value: "personal", Label: "Personal projects"},
						{Value: "education", Label: "Education"},
						{Value: "other", Label: "Something else"},
					},
				},
				{
					Key: "found_us", Label: "How did you hear about us?", Type: models.FieldSelect, Display: models.DisplayDropdown,
					Options: []models.FieldOption{
						{Value: "search", Label: "Search engine"},
						{Value: "friend", Label: "A friend or colleague"},
						{Value: "social", Label: "Social media"},
						{Value: "other", Label: "Other"},
					},
				},
				{Key: "improve", Label: "What could we do better?", Type: models.FieldTextarea, MaxLen: intPtr(2000)},
				followUpEmail(),
			},
		},
		{
			DisplayName:  "Event registration",
			Name:         "event-registration",
			Description:  "A registration form over three pages, one section each.",
			Mode:         models.FormModeCollect,
			Confirmation: "You're registered. We'll send the details before the event.",
			Fields: models.FieldDefs{
				{Key: "about_you", Label: "About you", Type: models.FieldSection, Help: "Who is coming?"},
				{Key: "name", Label: "Full name", Type: models.FieldText, Required: true, Role: models.RoleSenderName, PII: true, MaxLen: intPtr(120)},
				senderEmail(),
				{Key: "company", Label: "Company", Type: models.FieldText, MaxLen: intPtr(120)},

				{Key: "attendance", Label: "Attendance", Type: models.FieldSection, Help: "Tell us how you'll take part."},
				{
					Key: "ticket", Label: "How will you attend?", Type: models.FieldSelect, Display: models.DisplayRadio, Required: true,
					Options: []models.FieldOption{
						{Value: "in_person", Label: "In person"},
						{Value: "online", Label: "Online"},
					},
				},
				{
					Key: "sessions", Label: "Which sessions interest you?", Type: models.FieldMultiSelect, Display: models.DisplayCheckboxes,
					Options: []models.FieldOption{
						{Value: "keynote", Label: "Keynote"},
						{Value: "workshops", Label: "Workshops"},
						{Value: "networking", Label: "Networking"},
					},
				},
				{Key: "arrival", Label: "Arrival date", Type: models.FieldDate},

				{Key: "details", Label: "Final details", Type: models.FieldSection},
				{Key: "dietary", Label: "Dietary requirements", Type: models.FieldText, MaxLen: intPtr(200)},
				{Key: "notes", Label: "Anything else we should know?", Type: models.FieldTextarea, MaxLen: intPtr(2000)},
				{Key: "consent", Label: "I agree to be contacted about this event", Type: models.FieldCheckbox, Required: true},
			},
		},
	}
}

// seedForms creates the example forms in a workspace that has none.
func (s *Seeder) seedForms(workspaceID, userID uint) {
	if s.formRepo == nil {
		return
	}
	scope := repositories.ResourceScope{UserID: userID, WorkspaceID: &workspaceID}
	if n, err := s.formRepo.CountByScope(scope); err != nil || n > 0 {
		return
	}
	for _, def := range defaultFormDefs() {
		key, err := models.NewFormPublicKey()
		if err != nil {
			logger.Error("failed to generate form key", "name", def.DisplayName, "error", err)
			return
		}
		notifyMode := models.NotifyModeImmediate
		if def.Mode == models.FormModeCollect {
			// As for a collect form created in the dashboard: one email a day.
			notifyMode = models.NotifyModeDaily
		}
		form := &models.Form{
			WorkspaceID:         workspacePtr(workspaceID),
			Name:                def.Name,
			DisplayName:         def.DisplayName,
			Description:         def.Description,
			PublicKey:           key,
			Status:              models.FormStatusActive,
			Mode:                def.Mode,
			MaxBodyBytes:        models.DefaultMaxBodyBytes,
			MaxFields:           models.DefaultMaxFields,
			Fields:              def.Fields,
			SchemaVersion:       1,
			UnknownFields:       models.UnknownFieldsKeep,
			ConfirmationMessage: def.Confirmation,
			HoneypotField:       models.DefaultHoneypotField,
			MinFillSeconds:      3,
			ScanEnabled:         true,
			FlagThreshold:       3,
			QuarantineThreshold: 6,
			RejectThreshold:     10,
			NotifyEnabled:       true,
			NotifyMode:          notifyMode,
			NotifyOnFlagged:     true,
			LastEditedByID:      &userID,
		}
		if err := s.formRepo.Create(form); err != nil {
			logger.Error("failed to seed form", "name", def.DisplayName, "workspace_id", workspaceID, "error", err)
		}
	}
}

// senderEmail is the required email question a reply goes to.
func senderEmail() models.FieldDef {
	return models.FieldDef{Key: "email", Label: "Email", Type: models.FieldEmail, Required: true, Role: models.RoleSenderEmail, PII: true}
}

// followUpEmail is an optional email question, for a survey that is
// anonymous unless the respondent wants an answer.
func followUpEmail() models.FieldDef {
	f := senderEmail()
	f.Required, f.Role, f.Help = false, models.RoleNone, "Only if you'd like us to follow up."
	return f
}

func intPtr(n int) *int           { return &n }
func floatPtr(n float64) *float64 { return &n }
