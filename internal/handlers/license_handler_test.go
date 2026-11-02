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

package handlers

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/goposta/posta/internal/enterprise"
	"github.com/goposta/posta/internal/enterprise/enterprisetest"
	"github.com/goposta/posta/internal/models"
)

func branchingSchema(next string) models.FieldDefs {
	return models.FieldDefs{
		{Key: "kind", Label: "I am", Type: models.FieldSelect, Options: []models.FieldOption{
			{Value: "company", Label: "A company"},
			{Value: "individual", Label: "An individual", Goto: "about"},
		}},
		{Key: "company", Label: "Company", Type: models.FieldSection, Next: next},
		{Key: "company_name", Label: "Company name", Type: models.FieldText},
		{Key: "about", Label: "About you", Type: models.FieldSection},
		{Key: "bio", Label: "Bio", Type: models.FieldTextarea},
	}
}

func plainSchema() models.FieldDefs {
	defs := branchingSchema("")
	defs[0].Options[1].Goto = ""
	return defs
}

func TestBranchingNeedsALicense(t *testing.T) {
	apply := func(current, next models.FieldDefs, ee enterprise.EE) error {
		form := &models.Form{Fields: current}
		return applySchema(form, &next, nil, nil, ee)
	}
	branching := branchingSchema("")
	edited := branchingSchema("")
	edited[2].Label = "Company legal name"

	cases := []struct {
		name          string
		current, next models.FieldDefs
		ee            enterprise.EE
		want          error
	}{
		{"add branching without a license", plainSchema(), branching, enterprisetest.Community, enterprise.ErrLicenseRequired},
		{"add branching with no edition wired", plainSchema(), branching, nil, enterprise.ErrLicenseRequired},
		{"add branching with an expired license", plainSchema(), branching, enterprisetest.Degraded, enterprise.ErrLicenseExpired},
		{"add branching with a license", plainSchema(), branching, enterprisetest.Licensed, nil},
		{"change a branch without a license", branching, branchingSchema("about"), enterprisetest.Community, enterprise.ErrLicenseRequired},
		{"edit a label of a branching form without a license", branching, edited, enterprisetest.Community, nil},
		{"remove branching without a license", branching, plainSchema(), enterprisetest.Community, nil},
		{"a form without branching", nil, plainSchema(), enterprisetest.Community, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := apply(tc.current, tc.next, tc.ee); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLicenseWarnings(t *testing.T) {
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	soon, later := now.AddDate(0, 0, 10), now.AddDate(1, 0, 0)
	cases := []struct {
		status enterprise.Status
		want   []string
	}{
		{enterprise.Status{State: enterprise.StateNone}, []string{}},
		{enterprise.Status{State: enterprise.StateValid, NotAfter: &later}, []string{}},
		{enterprise.Status{State: enterprise.StateValid, NotAfter: &soon}, []string{"nearing_expiry"}},
		{enterprise.Status{State: enterprise.StateGrace}, []string{"license_grace"}},
		{enterprise.Status{State: enterprise.StateDegraded}, []string{"license_expired"}},
		{enterprise.Status{State: enterprise.StateBindingMismatch}, []string{"binding_mismatch"}},
		{enterprise.Status{State: enterprise.StateUnknownKey}, []string{"unknown_signing_key"}},
	}
	for _, tc := range cases {
		if got := licenseWarnings(tc.status, now); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.status.State, got, tc.want)
		}
	}
}
