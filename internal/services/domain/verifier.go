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

package domain

import (
	"net"
	"strings"

	"github.com/goposta/posta/internal/models"
)

const recordTypeTXT = "TXT"

type VerificationResult struct {
	OwnershipVerified bool   `json:"ownership_verified"`
	SPFVerified       bool   `json:"spf_verified"`
	DKIMVerified      bool   `json:"dkim_verified"`
	DMARCVerified     bool   `json:"dmarc_verified"`
	SPFRecord         string `json:"spf_record,omitempty"`
	DKIMRecord        string `json:"dkim_record,omitempty"`
	DMARCRecord       string `json:"dmarc_record,omitempty"`
}

type DNSRecords struct {
	Verification DNSRecord `json:"verification"`
	SPF          DNSRecord `json:"spf"`
	DKIM         DNSRecord `json:"dkim"`
	DMARC        DNSRecord `json:"dmarc"`
}

type DNSRecord struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Value string `json:"value"`
}

func RequiredRecords(d *models.Domain) *DNSRecords {
	return &DNSRecords{
		Verification: DNSRecord{
			Type:  recordTypeTXT,
			Host:  d.Domain,
			Value: "posta-verification=" + d.VerificationToken,
		},
		SPF: DNSRecord{
			Type:  recordTypeTXT,
			Host:  d.Domain,
			Value: "v=spf1 include:_spf.posta ~all",
		},
		DKIM: DNSRecord{
			Type:  "CNAME",
			Host:  "posta._domainkey." + d.Domain,
			Value: "posta._domainkey.posta",
		},
		DMARC: DNSRecord{
			Type:  recordTypeTXT,
			Host:  "_dmarc." + d.Domain,
			Value: "v=DMARC1; p=none; rua=mailto:dmarc@" + d.Domain,
		},
	}
}

func Verify(d *models.Domain) (*VerificationResult, error) {
	result := &VerificationResult{}

	ownershipVerified := false
	txtRecords, err := net.LookupTXT(d.Domain)
	if err == nil {
		expectedToken := "posta-verification=" + d.VerificationToken
		for _, txt := range txtRecords {
			if strings.TrimSpace(txt) == expectedToken {
				ownershipVerified = true
			}
			if strings.HasPrefix(strings.TrimSpace(txt), "v=spf1") {
				result.SPFVerified = true
				result.SPFRecord = txt
			}
		}
	}

	result.OwnershipVerified = ownershipVerified
	if !ownershipVerified {
		return result, nil
	}

	dkimHost := "posta._domainkey." + d.Domain
	dkimRecords, err := net.LookupTXT(dkimHost)
	if err == nil && len(dkimRecords) > 0 {
		result.DKIMVerified = true
		result.DKIMRecord = dkimRecords[0]
	}
	if !result.DKIMVerified {
		cname, err := net.LookupCNAME(dkimHost)
		if err == nil && cname != "" {
			result.DKIMVerified = true
			result.DKIMRecord = cname
		}
	}

	dmarcHost := "_dmarc." + d.Domain
	dmarcRecords, err := net.LookupTXT(dmarcHost)
	if err == nil {
		for _, txt := range dmarcRecords {
			if strings.HasPrefix(strings.TrimSpace(txt), "v=DMARC1") {
				result.DMARCVerified = true
				result.DMARCRecord = txt
				break
			}
		}
	}

	return result, nil
}
