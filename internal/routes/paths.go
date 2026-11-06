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

// Route paths registered by more than one definition, typically the GET, PUT,
// and DELETE of a single resource. They are named rather than repeated so a
// change to one method's path cannot leave the others behind, which would turn
// into a 404 on exactly one verb.
//
// They live together because goconst counts occurrences per package, not per
// file, and several are shared across route files.
const (
	pathBounces       = "/bounces"
	pathNotifications = "/notifications"
	pathAnnouncements = "/announcements"
	pathDomains       = "/domains"
	pathSettings      = "/settings"
	pathSuppressions  = "/suppressions"
	pathWebhooks      = "/webhooks"
	pathWorkspaces    = "/workspaces"

	pathByID                  = "/{id:int}"
	pathAnnouncementByID      = "/announcements/{id:int}"
	pathCampaignByID          = "/campaigns/{id:int}"
	pathChatWidgetByID        = "/chat-widgets/{id:int}"
	pathConversationByID      = "/conversations/{id}"
	pathChatConversations     = "/{key}/conversations"
	pathChatMessages          = "/{key}/conversations/{id}/messages"
	pathDomainByID            = "/domains/{id:int}"
	pathFormByID              = "/forms/{id:int}"
	pathPlanByID              = "/plans/{id:int}"
	pathServerByID            = "/servers/{id:int}"
	pathSMTPServerByID        = "/smtp-servers/{id:int}"
	pathSubscriberByID        = "/subscribers/{id:int}"
	pathSubscriberListByID    = "/subscriber-lists/{id:int}"
	pathSubscriberListMembers = "/subscriber-lists/{id:int}/members"
	pathTeamByID              = "/teams/{id:int}"
	pathUnsubscribeListByID   = "/unsubscribe-lists/{id:int}"
	pathWorkspaceByID         = "/workspaces/{id:int}"
)
