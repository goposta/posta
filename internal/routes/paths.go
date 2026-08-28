// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package routes

// Route paths registered by more than one definition, typically the GET, PUT,
// and DELETE of a single resource. They are named rather than repeated so a
// change to one method's path cannot leave the others behind, which would turn
// into a 404 on exactly one verb.
//
// They live together because goconst counts occurrences per package, not per
// file, and several are shared across route files.
const (
	pathBounces      = "/bounces"
	pathSettings     = "/settings"
	pathSuppressions = "/suppressions"
	pathWebhooks     = "/webhooks"

	pathCampaignByID          = "/campaigns/{id:int}"
	pathFormByID              = "/forms/{id:int}"
	pathPlanByID              = "/plans/{id:int}"
	pathServerByID            = "/servers/{id:int}"
	pathSMTPServerByID        = "/smtp-servers/{id:int}"
	pathSubscriberByID        = "/subscribers/{id:int}"
	pathSubscriberListByID    = "/subscriber-lists/{id:int}"
	pathSubscriberListMembers = "/subscriber-lists/{id:int}/members"
	pathUnsubscribeListByID   = "/unsubscribe-lists/{id:int}"
)
