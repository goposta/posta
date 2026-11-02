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
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/goposta/posta/internal/middlewares"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

// Form submissions are a form's messages read as responses: typed answers,
// field filters, a summary and an export. They live outside /forms on
// purpose: /forms is tenant administration (admin scope), while reading
// responses is reading content (read scope).

const (
	// maxSummarySubmissions bounds how many of the newest submissions a
	// summary reads; the result says when it stopped short.
	maxSummarySubmissions = 50000
	// maxSubmissionExport bounds one export.
	maxSubmissionExport = 100000
)

// SubmissionQueryRequest selects a form's submissions. Field filters are read
// from the raw query: field.<key>=value for equality, field.<key>[gte]=n and
// field.<key>[lte]=n for ranges on number, scale, rating, date and time
// questions.
type SubmissionQueryRequest struct {
	FormID   int    `query:"form_id" doc:"The form whose submissions to read (required)"`
	Page     int    `query:"page" default:"0"`
	Size     int    `query:"size" default:"20"`
	Status   string `query:"status" enum:"received,flagged,quarantined,rejected" doc:"Defaults to received and flagged: spam is not a response"`
	Q        string `query:"q" doc:"Free-text search over the submission"`
	After    string `query:"after" doc:"RFC 3339 lower bound on received time"`
	Before   string `query:"before" doc:"RFC 3339 upper bound on received time"`
	Tag      string `query:"tag" doc:"Only submissions carrying this tag"`
	Priority string `query:"priority" enum:"low,normal,high,urgent"`
	Format   string `query:"format" enum:"csv,json" default:"csv" doc:"Export only"`
}

// submissionQuery resolves the form and the filter a request names. It writes
// the error response itself and returns nil when the request is invalid.
func (h *MessageHandler) submissionQuery(c *okapi.Context, req *SubmissionQueryRequest) (*models.Form, *repositories.MessageFilterQuery, error) {
	if req.FormID <= 0 {
		return nil, nil, c.AbortBadRequest("form_id is required")
	}
	form, err := h.formRepo.FindByIDForScope(getScope(c), uint(req.FormID))
	if err != nil {
		return nil, nil, c.AbortNotFound("form not found")
	}
	equals, ranges, err := parseFieldFilters(form.Fields, c.Request().URL.Query())
	if err != nil {
		return nil, nil, c.AbortBadRequest(err.Error())
	}
	tags, err := tagFilter(req.Tag)
	if err != nil {
		return nil, nil, c.AbortBadRequest(err.Error())
	}
	filter := &repositories.MessageFilterQuery{
		FormID:          form.ID,
		Status:          req.Status,
		Query:           strings.TrimSpace(req.Q),
		Tags:            tags,
		Priority:        req.Priority,
		SubmissionsOnly: true,
		DataEquals:      equals,
		DataRanges:      ranges,
	}
	if t, err := time.Parse(time.RFC3339, req.After); err == nil {
		filter.From = &t
	}
	if t, err := time.Parse(time.RFC3339, req.Before); err == nil {
		filter.To = &t
	}
	return form, filter, nil
}

// parseFieldFilters turns field.<key>[op]=value query parameters into typed
// filters. Every key must be a question of the form, so nothing a caller
// sends reaches SQL except as a bound value.
func parseFieldFilters(defs models.FieldDefs, query url.Values) (map[string]any, []repositories.DataRange, error) {
	var equals map[string]any
	var ranges []repositories.DataRange
	for param, values := range query {
		if !strings.HasPrefix(param, "field.") || len(values) == 0 {
			continue
		}
		key, op := strings.TrimPrefix(param, "field."), ""
		if i := strings.IndexByte(key, '['); i >= 0 && strings.HasSuffix(key, "]") {
			key, op = key[:i], key[i+1:len(key)-1]
		}
		if len(defs) == 0 {
			return nil, nil, fmt.Errorf("this form has no questions to filter by")
		}
		var def *models.FieldDef
		for i := range defs {
			if defs[i].Key == key && !defs[i].IsSection() {
				def = &defs[i]
				break
			}
		}
		if def == nil {
			return nil, nil, fmt.Errorf("unknown field %q", key)
		}
		raw := strings.TrimSpace(values[0])
		numeric := def.Type == models.FieldNumber || def.Type == models.FieldScale || def.Type == models.FieldRating

		switch op {
		case "":
			v, err := equalityValue(def, raw)
			if err != nil {
				return nil, nil, err
			}
			if equals == nil {
				equals = map[string]any{}
			}
			equals[key] = v
		case "gte", "lte":
			r := repositories.DataRange{Key: key, Op: op, Numeric: numeric}
			switch {
			case numeric:
				n, err := strconv.ParseFloat(raw, 64)
				if err != nil {
					return nil, nil, fmt.Errorf("field %q: %q is not a number", key, raw)
				}
				r.Value = n
			case def.Type == models.FieldDate || def.Type == models.FieldTime:
				r.Value = raw
			default:
				return nil, nil, fmt.Errorf("field %q cannot be filtered by range", key)
			}
			ranges = append(ranges, r)
		default:
			return nil, nil, fmt.Errorf("field %q: unknown operator %q (use gte or lte)", key, op)
		}
	}
	return equals, ranges, nil
}

// equalityValue types a filter value the way submissions store it, so the
// containment match is exact.
func equalityValue(def *models.FieldDef, raw string) (any, error) {
	switch def.Type {
	case models.FieldNumber, models.FieldScale, models.FieldRating:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("field %q: %q is not a number", def.Key, raw)
		}
		return n, nil
	case models.FieldCheckbox:
		switch strings.ToLower(raw) {
		case "true", "yes", "1", "on":
			return true, nil
		case "false", "no", "0", "off":
			return false, nil
		}
		return nil, fmt.Errorf("field %q: %q is not true or false", def.Key, raw)
	case models.FieldMultiSelect:
		return []any{raw}, nil
	}
	return raw, nil
}

// canSeePII reports whether the caller may read personal-data answers in
// bulk: a workspace owner or admin, or an API key with the admin scope. A
// workspace-bound key resolves as owner, so its scopes decide, not its role.
func canSeePII(c *okapi.Context) bool {
	if c.GetString(middlewares.CtxAuthMethod) == middlewares.AuthMethodAPIKey {
		for _, s := range middlewares.APIKeyScopes(c) {
			if s == models.ScopeAdmin || s == models.ScopeAll {
				return true
			}
		}
		return false
	}
	return workspaceRole(c).CanManageMembers()
}

// ListSubmissions pages through a form's submissions, newest first.
func (h *MessageHandler) ListSubmissions(c *okapi.Context, req *SubmissionQueryRequest) error {
	_, filter, err := h.submissionQuery(c, req)
	if filter == nil {
		return err
	}
	page, size, offset := normalizePageParams(req.Page, req.Size)
	items, total, err := h.repo.FindByScopeFiltered(getScope(c), *filter, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list submissions")
	}
	for i := range items {
		hydrate(&items[i])
		items[i].Body = truncateBody(items[i].Body, 400)
		items[i].Form = nil
	}
	return paginated(c, items, total, page, size)
}

// SubmissionSummary reduces a form's submissions to per-question figures.
func (h *MessageHandler) SubmissionSummary(c *okapi.Context, req *SubmissionQueryRequest) error {
	form, filter, err := h.submissionQuery(c, req)
	if filter == nil {
		return err
	}
	s := messages.NewSummarizer(form.Fields, canSeePII(c))
	truncated, err := h.repo.EachMessage(getScope(c), *filter, 1000, maxSummarySubmissions, func(batch []models.Message) error {
		for i := range batch {
			s.Add(answersOf(&batch[i]))
		}
		return nil
	})
	if err != nil {
		return c.AbortInternalServerError("failed to summarize submissions")
	}
	return ok(c, s.Result(truncated))
}

// answersOf returns a submission's answers by key. Submissions stored before
// the form had a schema have no typed data, so their raw fields stand in.
func answersOf(m *models.Message) models.SubmissionData {
	if m.Data != nil {
		return m.Data
	}
	hydrate(m)
	out := make(models.SubmissionData, len(m.Fields))
	for _, f := range m.Fields {
		if _, seen := out[f.Key]; !seen {
			out[f.Key] = f.Value
		}
	}
	return out
}

// exportColumns are the questions an export carries, active ones first, then
// archived ones, so old answers are not lost. Personal-data questions are
// left out unless the caller may see them.
func priorityOf(m *models.Message) models.MessagePriority {
	if m.Priority == "" {
		return models.MessagePriorityNormal
	}
	return m.Priority
}

func tagsOf(m *models.Message) []string {
	if m.Tags == nil {
		return []string{}
	}
	return m.Tags
}

func exportColumns(defs models.FieldDefs, showPII bool) (cols []models.FieldDef, withheld int) {
	for _, archived := range []bool{false, true} {
		for _, f := range defs {
			if f.Archived != archived || f.IsSection() {
				continue
			}
			if f.PII && !showPII {
				withheld++
				continue
			}
			cols = append(cols, f)
		}
	}
	return cols, withheld
}

// csvSafe keeps a cell from being read as a formula by a spreadsheet:
// submissions are written by the public.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// ExportSubmissions streams a form's submissions as CSV or JSON.
func (h *MessageHandler) ExportSubmissions(c *okapi.Context, req *SubmissionQueryRequest) error {
	form, filter, err := h.submissionQuery(c, req)
	if filter == nil {
		return err
	}
	format := req.Format
	if format != "json" {
		format = "csv"
	}
	showPII := canSeePII(c)
	cols, withheld := exportColumns(form.Fields, showPII)
	known := map[string]bool{form.Honeypot(): true}
	for _, f := range form.Fields {
		known[f.Key] = true
	}

	slug := form.Slug
	if slug == "" {
		slug = "form"
	}
	filename := fmt.Sprintf("%s-responses-%s.%s", slug, time.Now().UTC().Format("20060102"), format)
	hdr := c.ResponseWriter().Header()
	if format == "csv" {
		hdr.Set("Content-Type", "text/csv; charset=utf-8")
	} else {
		hdr.Set("Content-Type", "application/json; charset=utf-8")
	}
	hdr.Set("Content-Disposition", contentDisposition(filename))
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("X-Content-Type-Options", "nosniff")
	if withheld > 0 {
		hdr.Set("X-Posta-Withheld-Fields", strconv.Itoa(withheld))
	}
	c.ResponseWriter().WriteHeader(http.StatusOK)

	out := bufio.NewWriter(c.ResponseWriter())
	defer func() { _ = out.Flush() }()

	rows := 0
	var write func(m *models.Message) error
	var finish func() error

	if format == "csv" {
		// The byte-order mark makes Excel read the file as UTF-8.
		_, _ = out.WriteString("\ufeff")
		w := csv.NewWriter(out)
		header := []string{"Submitted at", "ID", "Status", "Priority", "Tags"}
		if len(form.Fields) == 0 {
			header = append(header, "Email", "Name", "Phone", "Subject", "Message", "Fields")
		}
		for _, f := range cols {
			label := f.Label
			if label == "" {
				label = f.Key
			}
			if f.Archived {
				label += " (archived)"
			}
			header = append(header, label)
		}
		if len(form.Fields) > 0 {
			header = append(header, "Other fields")
		}
		if err := w.Write(header); err != nil {
			return nil
		}
		write = func(m *models.Message) error {
			record := []string{m.CreatedAt.UTC().Format(time.RFC3339), m.UUID, string(m.Status),
				string(priorityOf(m)), csvSafe(strings.Join(m.Tags, "; "))}
			if len(form.Fields) == 0 {
				hydrate(m)
				raw, _ := json.Marshal(m.Fields)
				record = append(record, csvSafe(m.SenderEmail), csvSafe(m.SenderName), csvSafe(m.SenderPhone),
					csvSafe(m.Subject), csvSafe(m.Body), csvSafe(string(raw)))
			}
			answers := answersOf(m)
			for i := range cols {
				cell := messages.ExportValue(&cols[i], answers[cols[i].Key])
				switch cols[i].Type {
				case models.FieldNumber, models.FieldScale, models.FieldRating:
				default:
					cell = csvSafe(cell)
				}
				record = append(record, cell)
			}
			if len(form.Fields) > 0 {
				record = append(record, csvSafe(otherFields(m, known)))
			}
			return w.Write(record)
		}
		finish = func() error {
			w.Flush()
			return w.Error()
		}
	} else {
		_, _ = out.WriteString("[")
		write = func(m *models.Message) error {
			item := map[string]any{
				"id":           m.UUID,
				"submitted_at": m.CreatedAt.UTC().Format(time.RFC3339),
				"status":       m.Status,
				"priority":     priorityOf(m),
				"tags":         tagsOf(m),
			}
			if len(form.Fields) == 0 {
				hydrate(m)
				item["sender_email"], item["sender_name"], item["subject"], item["body"] = m.SenderEmail, m.SenderName, m.Subject, m.Body
				item["fields"] = m.Fields
			} else {
				answers := answersOf(m)
				data := make(map[string]any, len(cols))
				for _, f := range cols {
					if v, ok := answers[f.Key]; ok {
						data[f.Key] = v
					}
				}
				item["data"] = data
				if other := otherFields(m, known); other != "" {
					item["other_fields"] = json.RawMessage(other)
				}
			}
			raw, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if rows > 0 {
				_, _ = out.WriteString(",")
			}
			_, err = out.Write(raw)
			return err
		}
		finish = func() error {
			_, err := out.WriteString("]\n")
			return err
		}
	}

	truncated, err := h.repo.EachMessage(getScope(c), *filter, 500, maxSubmissionExport, func(batch []models.Message) error {
		for i := range batch {
			if err := write(&batch[i]); err != nil {
				return err
			}
			rows++
		}
		return nil
	})
	if err == nil {
		err = finish()
	}
	if err != nil {
		// The status is already sent; the file ends short.
		logger.Warn("form submission export failed", "form_id", form.ID, "rows", rows, "error", err)
	}

	if h.audit != nil {
		if scope := getScope(c); scope.WorkspaceID != nil {
			piiIncluded := false
			for _, f := range cols {
				piiIncluded = piiIncluded || f.PII
			}
			h.audit.LogCtxScoped(c, *scope.WorkspaceID, "form.submissions_exported",
				fmt.Sprintf("Exported %d responses of form %q as %s", rows, form.Name, strings.ToUpper(format)),
				map[string]any{"form_id": form.ID, "format": format, "rows": rows, "pii": piiIncluded, "withheld_fields": withheld, "truncated": truncated})
		}
	}
	return nil
}

// otherFields returns, as a JSON object, the submitted fields the schema does
// not declare (kept under the "keep" policy), or "" when there are none.
func otherFields(m *models.Message, known map[string]bool) string {
	hydrate(m)
	extra := map[string]string{}
	for _, f := range m.Fields {
		if !known[f.Key] && !strings.HasPrefix(f.Key, "_") {
			extra[f.Key] = f.Value
		}
	}
	if len(extra) == 0 {
		return ""
	}
	raw, _ := json.Marshal(extra)
	return string(raw)
}
