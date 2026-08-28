// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package routes

// OpenAPI tag names. A tag groups routes in the generated specification, so
// every route in a group has to spell it identically — renaming one of a dozen
// literals silently splits the group in two. They live here because several are
// used from more than one route file.
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

// Tag descriptions that appear in more than one route file. Duplicated prose
// drifts; a group whose description differs per file renders unpredictably.
const (
	descAdmin             = "Platform-level administration: users, workspaces, global settings, OAuth providers, and live event streams. Admin-only."
	descOAuth             = "Single sign-on via external identity providers (Google, GitHub, generic OIDC). Covers public flow, user-account linking, admin provider config, and workspace SSO enforcement."
	descWorkspacesWithSSO = "Create workspaces, manage members and invitations, and configure workspace-scoped settings (including SSO)."
)
