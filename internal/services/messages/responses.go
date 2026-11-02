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

package messages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goposta/posta/internal/models"
)

// summarySamples is how many recent text answers a question's summary shows.
const summarySamples = 20

// OptionCount is how many responses chose one value.
type OptionCount struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// FieldSummary aggregates the answers to one question.
type FieldSummary struct {
	Key      string           `json:"key"`
	Label    string           `json:"label"`
	Type     models.FieldType `json:"type"`
	PII      bool             `json:"pii,omitempty"`
	Answered int              `json:"answered"`
	// Options counts each value of a choice, checkbox, scale or rating
	// question, in the schema's order, then any value the schema no longer has.
	Options []OptionCount `json:"options,omitempty"`
	Mean    *float64      `json:"mean,omitempty"`
	Min     *float64      `json:"min,omitempty"`
	Max     *float64      `json:"max,omitempty"`
	// Samples are the latest text answers, newest first. SamplesHidden says
	// they were withheld because the question is personal data.
	Samples       []string `json:"samples,omitempty"`
	SamplesHidden bool     `json:"samples_hidden,omitempty"`
}

// SubmissionSummary is a form's responses reduced to per-question figures.
type SubmissionSummary struct {
	Total     int            `json:"total"`
	Truncated bool           `json:"truncated"`
	Fields    []FieldSummary `json:"fields"`
}

type fieldAgg struct {
	def    models.FieldDef
	sum    FieldSummary
	counts map[string]int
	extra  []string // values seen that the schema does not list, in first-seen order
	total  float64
	nums   int
}

// Summarizer builds a SubmissionSummary from submissions fed newest first.
type Summarizer struct {
	fields  []*fieldAgg
	total   int
	showPII bool
}

// NewSummarizer summarizes the active questions of defs. Without showPII, the
// text answers of personal-data questions are counted but not quoted.
func NewSummarizer(defs models.FieldDefs, showPII bool) *Summarizer {
	s := &Summarizer{showPII: showPII}
	for _, f := range defs.Active() {
		if f.Type == models.FieldHidden || f.IsSection() {
			continue
		}
		s.fields = append(s.fields, &fieldAgg{
			def:    f,
			sum:    FieldSummary{Key: f.Key, Label: f.Label, Type: f.Type, PII: f.PII},
			counts: map[string]int{},
		})
	}
	return s
}

// Add counts one submission's answers.
func (s *Summarizer) Add(data models.SubmissionData) {
	s.total++
	for _, a := range s.fields {
		v, ok := data[a.def.Key]
		if !ok || isEmptyAnswer(v) {
			continue
		}
		a.sum.Answered++
		switch a.def.Type {
		case models.FieldSelect, models.FieldMultiSelect:
			for _, item := range answerList(v) {
				a.count(item)
			}
		case models.FieldCheckbox:
			if b, isBool := v.(bool); isBool && b {
				a.count("true")
			} else {
				a.count("false")
			}
		case models.FieldScale, models.FieldRating, models.FieldNumber:
			n, isNum := v.(float64)
			if !isNum {
				continue
			}
			a.total += n
			a.nums++
			if a.sum.Min == nil || n < *a.sum.Min {
				a.sum.Min = ptr(n)
			}
			if a.sum.Max == nil || n > *a.sum.Max {
				a.sum.Max = ptr(n)
			}
			if a.def.Type != models.FieldNumber {
				a.count(strconv.FormatFloat(n, 'f', -1, 64))
			}
		default:
			if len(a.sum.Samples) < summarySamples && (!a.def.PII || s.showPII) {
				a.sum.Samples = append(a.sum.Samples, fmt.Sprint(v))
			}
		}
	}
}

func (a *fieldAgg) count(value string) {
	if _, seen := a.counts[value]; !seen && !a.known(value) {
		a.extra = append(a.extra, value)
	}
	a.counts[value]++
}

func (a *fieldAgg) known(value string) bool {
	for _, o := range a.choices() {
		if o.Value == value {
			return true
		}
	}
	return false
}

// choices lists the values a question offers, in display order.
func (a *fieldAgg) choices() []OptionCount {
	f := &a.def
	switch f.Type {
	case models.FieldSelect, models.FieldMultiSelect:
		out := make([]OptionCount, 0, len(f.Options))
		for _, o := range f.Options {
			label := o.Label
			if label == "" {
				label = o.Value
			}
			out = append(out, OptionCount{Value: o.Value, Label: label})
		}
		return out
	case models.FieldCheckbox:
		return []OptionCount{{Value: "true", Label: "Yes"}, {Value: "false", Label: "No"}}
	case models.FieldScale, models.FieldRating:
		lo, hi := ScaleRange(f)
		if f.Type == models.FieldRating {
			lo, hi = 1, RatingStars(f)
		}
		out := make([]OptionCount, 0, hi-lo+1)
		for n := lo; n <= hi; n++ {
			v := strconv.Itoa(n)
			out = append(out, OptionCount{Value: v, Label: v})
		}
		return out
	}
	return nil
}

// Result returns the summary. truncated reports that only the newest
// submissions were read.
func (s *Summarizer) Result(truncated bool) SubmissionSummary {
	out := SubmissionSummary{Total: s.total, Truncated: truncated, Fields: make([]FieldSummary, 0, len(s.fields))}
	for _, a := range s.fields {
		sum := a.sum
		if choices := a.choices(); choices != nil {
			for _, c := range choices {
				c.Count = a.counts[c.Value]
				sum.Options = append(sum.Options, c)
			}
			for _, v := range a.extra {
				sum.Options = append(sum.Options, OptionCount{Value: v, Label: v, Count: a.counts[v]})
			}
		}
		if a.nums > 0 {
			sum.Mean = ptr(a.total / float64(a.nums))
		}
		if a.def.PII && !s.showPII && sum.Answered > 0 && sum.Options == nil && sum.Mean == nil {
			sum.SamplesHidden = true
		}
		out.Fields = append(out.Fields, sum)
	}
	return out
}

// ExportValue renders an answer for a spreadsheet cell: choices by their
// labels, booleans as yes or no, several values joined with "; ".
func ExportValue(f *models.FieldDef, v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "yes"
		}
		return "no"
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		return exportLabel(f, val)
	case []any:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, exportLabel(f, fmt.Sprint(item)))
		}
		return strings.Join(parts, "; ")
	}
	return fmt.Sprint(v)
}

func exportLabel(f *models.FieldDef, value string) string {
	if f == nil {
		return value
	}
	for _, o := range f.Options {
		if o.Value == value && o.Label != "" {
			return o.Label
		}
	}
	return value
}

func answerList(v any) []string {
	if list, ok := v.([]any); ok {
		out := make([]string, 0, len(list))
		for _, item := range list {
			out = append(out, fmt.Sprint(item))
		}
		return out
	}
	return []string{fmt.Sprint(v)}
}

func isEmptyAnswer(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(val) == ""
	case []any:
		return len(val) == 0
	}
	return false
}

func ptr[T any](v T) *T { return &v }
