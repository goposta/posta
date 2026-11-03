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
	"fmt"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/formrules"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// FormRuleHandler edits a form's routing rules. Rules are form configuration,
// so they need the admin scope like the form itself (/forms is a tenant
// admin path).
type FormRuleHandler struct {
	repo          *repositories.FormRuleRepository
	formRepo      *repositories.FormRepository
	messageRepo   *repositories.MessageRepository
	teamRepo      *repositories.TeamRepository
	workspaceRepo *repositories.WorkspaceRepository
	webhookRepo   *repositories.WebhookRepository
	audit         *audit.Logger
}

func NewFormRuleHandler(
	repo *repositories.FormRuleRepository,
	formRepo *repositories.FormRepository,
	messageRepo *repositories.MessageRepository,
	teamRepo *repositories.TeamRepository,
	workspaceRepo *repositories.WorkspaceRepository,
	webhookRepo *repositories.WebhookRepository,
	auditLogger *audit.Logger,
) *FormRuleHandler {
	return &FormRuleHandler{
		repo: repo, formRepo: formRepo, messageRepo: messageRepo, teamRepo: teamRepo,
		workspaceRepo: workspaceRepo, webhookRepo: webhookRepo, audit: auditLogger,
	}
}

// RuleBody is a rule as the dashboard edits it.
type RuleBody struct {
	Name       string                `json:"name" doc:"Shown in the rule list and on routed submissions"`
	Enabled    *bool                 `json:"enabled" doc:"Default true"`
	Match      models.RuleMatch      `json:"match" enum:"all,any" doc:"Whether all conditions must hold, or any one. Default all."`
	Conditions models.RuleConditions `json:"conditions" doc:"Empty matches every submission"`
	Actions    models.RuleActions    `json:"actions"`
	Stop       bool                  `json:"stop" doc:"Do not evaluate later rules when this one matches"`
}

type FormRulesRequest struct {
	ID int `param:"id"`
}

type CreateFormRuleRequest struct {
	ID   int      `param:"id"`
	Body RuleBody `json:"body"`
}

type UpdateFormRuleRequest struct {
	ID     int      `param:"id"`
	RuleID int      `param:"rule_id"`
	Body   RuleBody `json:"body"`
}

type FormRuleIDRequest struct {
	ID     int `param:"id"`
	RuleID int `param:"rule_id"`
}

type ReorderFormRulesRequest struct {
	ID   int `param:"id"`
	Body struct {
		IDs []uint `json:"ids" doc:"Every rule of the form, in the new evaluation order"`
	} `json:"body"`
}

type TestFormRulesRequest struct {
	ID   int `param:"id"`
	Body struct {
		Rules []RuleBody `json:"rules" doc:"Rules to try instead of the saved ones, in order; omit to test the saved rules"`
		Limit int        `json:"limit" doc:"How many recent submissions to replay, at most 200 (default 100)"`
	} `json:"body"`
}

// RuleTestResult is a dry run: what the rules would have done with recent
// submissions. Nothing is sent or changed.
type RuleTestResult struct {
	Scanned int              `json:"scanned"`
	Matched int              `json:"matched" doc:"Submissions at least one rule matched"`
	Rules   []RuleTestCount  `json:"rules"`
	Samples []RuleTestSample `json:"samples"`
}

type RuleTestCount struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Matches int    `json:"matches"`
}

type RuleTestSample struct {
	MessageUUID string            `json:"message_uuid"`
	Sender      string            `json:"sender"`
	Subject     string            `json:"subject"`
	ReceivedAt  time.Time         `json:"received_at"`
	Matched     []int             `json:"matched" doc:"Indexes into rules"`
	Outcome     formrules.Outcome `json:"outcome"`
}

func (h *FormRuleHandler) form(c *okapi.Context, id int) (*models.Form, error) {
	return h.formRepo.FindByIDForScope(getScope(c), uint(id))
}

// destinations lists what this workspace's rules may point at.
func (h *FormRuleHandler) destinations(workspaceID uint) (formrules.Destinations, error) {
	d := formrules.Destinations{Members: map[uint]bool{}, Teams: map[uint]bool{}, Webhooks: map[uint]bool{}}
	members, err := h.workspaceRepo.ListMembers(workspaceID)
	if err != nil {
		return d, err
	}
	for _, m := range members {
		d.Members[m.UserID] = true
	}
	teams, err := h.teamRepo.IDs(workspaceID)
	if err != nil {
		return d, err
	}
	for _, id := range teams {
		d.Teams[id] = true
	}
	hooks, _, err := h.webhookRepo.FindByWorkspaceID(workspaceID, 1000, 0)
	if err != nil {
		return d, err
	}
	for _, wh := range hooks {
		d.Webhooks[wh.ID] = true
	}
	return d, nil
}

func (h *FormRuleHandler) checked(c *okapi.Context, form *models.Form, b RuleBody) (*models.FormRule, error) {
	rule := &models.FormRule{
		Name: b.Name, Enabled: b.Enabled == nil || *b.Enabled, Match: b.Match,
		Conditions: b.Conditions, Actions: b.Actions, Stop: b.Stop,
	}
	if rule.Conditions == nil {
		rule.Conditions = models.RuleConditions{}
	}
	d, err := h.destinations(*form.WorkspaceID)
	if err != nil {
		return nil, c.AbortInternalServerError("failed to load workspace destinations")
	}
	if err := formrules.Check(rule, form, d); err != nil {
		return nil, c.AbortBadRequest(err.Error())
	}
	return rule, nil
}

func (h *FormRuleHandler) List(c *okapi.Context, req *FormRulesRequest) error {
	form, err := h.form(c, req.ID)
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	rules, err := h.repo.ListByForm(form.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to list rules")
	}
	if rules == nil {
		rules = []models.FormRule{}
	}
	return ok(c, rules)
}

func (h *FormRuleHandler) Create(c *okapi.Context, req *CreateFormRuleRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	form, err := h.form(c, req.ID)
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	rule, err := h.checked(c, form, req.Body)
	if rule == nil {
		return err
	}
	userID := uint(c.GetInt("user_id"))
	rule.WorkspaceID, rule.FormID, rule.LastEditedByID = form.WorkspaceID, form.ID, &userID
	if err := h.repo.Create(rule); err != nil {
		return c.AbortInternalServerError("failed to create rule")
	}
	h.log(c, "form.rule_created", fmt.Sprintf("Rule %q created on form %s", rule.Name, form.DisplayName), form, rule)
	return created(c, rule)
}

func (h *FormRuleHandler) Update(c *okapi.Context, req *UpdateFormRuleRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	form, err := h.form(c, req.ID)
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	existing, err := h.repo.FindForForm(form.ID, uint(req.RuleID))
	if err != nil {
		return c.AbortNotFound("rule not found")
	}
	rule, err := h.checked(c, form, req.Body)
	if rule == nil {
		return err
	}
	now := time.Now()
	userID := uint(c.GetInt("user_id"))
	existing.Name, existing.Enabled, existing.Match = rule.Name, rule.Enabled, rule.Match
	existing.Conditions, existing.Actions, existing.Stop = rule.Conditions, rule.Actions, rule.Stop
	existing.LastEditedByID, existing.UpdatedAt = &userID, &now
	if err := h.repo.Update(existing); err != nil {
		return c.AbortInternalServerError("failed to update rule")
	}
	h.log(c, "form.rule_updated", fmt.Sprintf("Rule %q updated on form %s", existing.Name, form.DisplayName), form, existing)
	return ok(c, existing)
}

func (h *FormRuleHandler) Delete(c *okapi.Context, req *FormRuleIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	form, err := h.form(c, req.ID)
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	rule, err := h.repo.FindForForm(form.ID, uint(req.RuleID))
	if err != nil {
		return c.AbortNotFound("rule not found")
	}
	if err := h.repo.Delete(rule.ID); err != nil {
		return c.AbortInternalServerError("failed to delete rule")
	}
	h.log(c, "form.rule_deleted", fmt.Sprintf("Rule %q deleted from form %s", rule.Name, form.DisplayName), form, rule)
	return noContent(c)
}

func (h *FormRuleHandler) Reorder(c *okapi.Context, req *ReorderFormRulesRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	form, err := h.form(c, req.ID)
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	if err := h.repo.Reorder(form.ID, req.Body.IDs); err != nil {
		return c.AbortBadRequest(err.Error())
	}
	rules, err := h.repo.ListByForm(form.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to list rules")
	}
	h.log(c, "form.rules_reordered", fmt.Sprintf("Rules reordered on form %s", form.DisplayName), form, nil)
	return ok(c, rules)
}

// Test replays recent submissions through the rules without acting on them.
func (h *FormRuleHandler) Test(c *okapi.Context, req *TestFormRulesRequest) error {
	form, err := h.form(c, req.ID)
	if err != nil {
		return c.AbortNotFound("form not found")
	}
	var rules []models.FormRule
	if req.Body.Rules != nil {
		for i, b := range req.Body.Rules {
			rule, err := h.checked(c, form, b)
			if rule == nil {
				return err
			}
			rule.UUID, rule.Position = fmt.Sprintf("draft-%d", i), i
			rules = append(rules, *rule)
		}
	} else if rules, err = h.repo.ListByForm(form.ID); err != nil {
		return c.AbortInternalServerError("failed to list rules")
	}

	limit := req.Body.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	sample, _, err := h.messageRepo.FindByScopeFiltered(getScope(c),
		repositories.MessageFilterQuery{FormID: form.ID, SubmissionsOnly: true}, limit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load submissions")
	}

	index := make(map[string]int, len(rules))
	resp := RuleTestResult{Scanned: len(sample), Rules: make([]RuleTestCount, len(rules)), Samples: []RuleTestSample{}}
	for i, r := range rules {
		index[r.UUID] = i
		resp.Rules[i] = RuleTestCount{Index: i, Name: r.Name}
	}
	for i := range sample {
		m := &sample[i]
		out := formrules.Evaluate(rules, formrules.FactsFor(m, form))
		if len(out.Matched) == 0 {
			continue
		}
		resp.Matched++
		hit := make([]int, 0, len(out.Matched))
		for _, id := range out.Matched {
			resp.Rules[index[id]].Matches++
			hit = append(hit, index[id])
		}
		if len(resp.Samples) < 20 {
			resp.Samples = append(resp.Samples, RuleTestSample{
				MessageUUID: m.UUID, Sender: senderOf(m), Subject: m.Subject,
				ReceivedAt: m.CreatedAt, Matched: hit, Outcome: out,
			})
		}
	}
	return ok(c, resp)
}

func senderOf(m *models.Message) string {
	switch {
	case m.SenderName != "" && m.SenderEmail != "":
		return fmt.Sprintf("%s <%s>", m.SenderName, m.SenderEmail)
	case m.SenderEmail != "":
		return m.SenderEmail
	}
	return m.SenderName
}

func (h *FormRuleHandler) log(c *okapi.Context, action, message string, form *models.Form, rule *models.FormRule) {
	if h.audit == nil {
		return
	}
	meta := map[string]any{metaFormID: form.ID}
	if rule != nil {
		meta["rule_id"], meta["rule_uuid"] = rule.ID, rule.UUID
	}
	h.audit.LogCtx(c, action, message, meta)
}
