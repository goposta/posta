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

package domain

import (
	"net"
	"strings"

	"github.com/jkaninda/posta/internal/models"
)

// VerificationResult holds the outcome of DNS record checks.
type VerificationResult struct {
	OwnershipVerified bool   `json:"ownership_verified"`
	SPFVerified       bool   `json:"spf_verified"`
	DKIMVerified      bool   `json:"dkim_verified"`
	DMARCVerified     bool   `json:"dmarc_verified"`
	SPFRecord         string `json:"spf_record,omitempty"`
	DKIMRecord        string `json:"dkim_record,omitempty"`
	DMARCRecord       string `json:"dmarc_record,omitempty"`
}

// DNSRecords returns the DNS records a user needs to configure for a domain.
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

// RequiredRecords returns the DNS records the user must add.
func RequiredRecords(d *models.Domain) *DNSRecords {
	return &DNSRecords{
		Verification: DNSRecord{
			Type:  "TXT",
			Host:  d.Domain,
			Value: "posta-verification=" + d.VerificationToken,
		},
		SPF: DNSRecord{
			Type:  "TXT",
			Host:  d.Domain,
			Value: "v=spf1 include:_spf.posta ~all",
		},
		DKIM: DNSRecord{
			Type:  "CNAME",
			Host:  "posta._domainkey." + d.Domain,
			Value: "posta._domainkey.posta",
		},
		DMARC: DNSRecord{
			Type:  "TXT",
			Host:  "_dmarc." + d.Domain,
			Value: "v=DMARC1; p=none; rua=mailto:dmarc@" + d.Domain,
		},
	}
}

// Verify performs DNS lookups to check SPF, DKIM, and DMARC records.
// It also checks the verification TXT record for domain ownership.
func Verify(d *models.Domain) (*VerificationResult, error) {
	result := &VerificationResult{}

	// Check domain ownership via TXT record
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

	// Check DKIM
	dkimHost := "posta._domainkey." + d.Domain
	dkimRecords, err := net.LookupTXT(dkimHost)
	if err == nil && len(dkimRecords) > 0 {
		result.DKIMVerified = true
		result.DKIMRecord = dkimRecords[0]
	}
	// Also check CNAME
	if !result.DKIMVerified {
		cname, err := net.LookupCNAME(dkimHost)
		if err == nil && cname != "" {
			result.DKIMVerified = true
			result.DKIMRecord = cname
		}
	}

	// Check DMARC
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
