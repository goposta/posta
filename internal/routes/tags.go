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

// Every route in a tag group must spell the tag identically, or renaming one literal silently splits the group.
const (
	tagAdmin                 = "Admin"
	tagCampaigns             = "Campaigns"
	tagForms                 = "Forms"
	tagInbound               = "Inbound"
	tagLanguages             = "Languages"
	tagMessages              = "Messages"
	tagOAuth                 = "OAuth"
	tagSubscriberLists       = "Subscriber Lists"
	tagSubscribers           = "Subscribers"
	tagTemplateLocalizations = "Template Localizations"
	tagTemplateVersions      = "Template Versions"
	tagTracking              = "Tracking"
	tagUnsubscribeLists      = "Unsubscribe Lists"
	tagUser                  = "User"
	tagWorkspaceResources    = "Workspace resources"
	tagWorkspaces            = "Workspaces"
)

// Shared so descriptions cannot drift: a group whose description differs per file renders unpredictably.
const (
	descAdmin             = "Platform-level administration: users, workspaces, global settings, OAuth providers, and live event streams. Admin-only."
	descOAuth             = "Single sign-on via external identity providers (Google, GitHub, generic OIDC). Covers public flow, user-account linking, admin provider config, and workspace SSO enforcement."
	descWorkspacesWithSSO = "Create workspaces, manage members and invitations, and configure workspace-scoped settings (including SSO)."
)
