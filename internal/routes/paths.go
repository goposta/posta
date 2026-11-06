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

package routes

// Paths shared by several methods are named so one verb's path cannot drift into a 404; goconst counts per package.
const (
	pathBounces       = "/bounces"
	pathNotifications = "/notifications"
	pathAnnouncements = "/announcements"
	pathDomains       = "/domains"
	pathSettings      = "/settings"
	pathSuppressions  = "/suppressions"
	pathWebhooks      = "/webhooks"
	pathWorkspaces    = "/workspaces"

	pathByID                  = "/{id}"
	pathAnnouncementByID      = "/announcements/{id}"
	pathCampaignByID          = "/campaigns/{id}"
	pathDomainByID            = "/domains/{id}"
	pathFormByID              = "/forms/{id}"
	pathPlanByID              = "/plans/{id}"
	pathServerByID            = "/servers/{id}"
	pathSMTPServerByID        = "/smtp-servers/{id}"
	pathSubscriberByID        = "/subscribers/{id}"
	pathSubscriberListByID    = "/subscriber-lists/{id}"
	pathSubscriberListMembers = "/subscriber-lists/{id}/members"
	pathTeamByID              = "/teams/{id}"
	pathTemplateByID          = "/templates/{id}"
	pathUnsubscribeListByID   = "/unsubscribe-lists/{id}"
	pathWorkspaceByID         = "/workspaces/{id}"
)
