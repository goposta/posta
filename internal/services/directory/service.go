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

// Package directory provisions Posta users from LDAP/AD sign-ins and syncs admin role and memberships from groups.
package directory

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/goposta/posta/internal/enterprise"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrAccountDisabled means the directory accepted the password but the Posta account is disabled.
	ErrAccountDisabled = errors.New("account is disabled")
	// ErrNoEmail means the directory entry has no email, a mapping mistake the admin must fix.
	ErrNoEmail = errors.New("the directory entry has no email address")
)

type Service struct {
	ee         enterprise.EE
	users      *repositories.UserRepository
	workspaces *repositories.WorkspaceRepository
	ldap       *repositories.LDAPRepository
}

func NewService(ee enterprise.EE, users *repositories.UserRepository, workspaces *repositories.WorkspaceRepository, ldap *repositories.LDAPRepository) *Service {
	return &Service{ee: ee, users: users, workspaces: workspaces, ldap: ldap}
}

// Login returns (nil, nil) when no directory knows the email, so the caller keeps its local result.
func (s *Service) Login(ctx context.Context, email, password string) (*models.User, error) {
	if s == nil || s.ee == nil {
		return nil, nil
	}
	a := s.ee.LDAP()
	if a == nil {
		return nil, nil // Community, or no license
	}
	ident, err := a.Authenticate(ctx, email, password)
	if errors.Is(err, enterprise.ErrLDAPNoMatch) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	user, err := s.provision(ident)
	if err != nil {
		return nil, err
	}
	// Group mapping is best effort: it must never block a valid sign-in.
	if err := s.reconcile(user, ident); err != nil {
		logger.Warn("directory: group mapping failed", "user", user.ID, "error", err)
	}
	return user, nil
}

// provision reuses an existing account with the same email, since the directory just proved ownership of it.
func (s *Service) provision(ident enterprise.LDAPIdentity) (*models.User, error) {
	email := strings.ToLower(strings.TrimSpace(ident.Email))
	if email == "" {
		return nil, ErrNoEmail
	}
	if u, err := s.users.FindByEmail(email); err == nil {
		if !u.Active {
			return nil, ErrAccountDisabled
		}
		return u, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	name := strings.TrimSpace(ident.Name)
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	now := time.Now()
	user := &models.User{
		Name:            name,
		Email:           email,
		PasswordHash:    unusablePassword(),
		Role:            models.UserRoleUser,
		Active:          true,
		AuthMethod:      models.AuthMethodLDAP,
		EmailVerifiedAt: &now, // the directory vouches for the address
	}
	if err := s.users.Create(user); err != nil {
		return nil, err
	}
	logger.Info("directory: account created on first sign-in", "user", user.ID, "directory", ident.Directory)
	return user, nil
}

// reconcile touches only directory-created accounts and only access a mapping governs; the rest stays as set by hand.
func (s *Service) reconcile(user *models.User, ident enterprise.LDAPIdentity) error {
	if user.AuthMethod != models.AuthMethodLDAP {
		return nil
	}
	cfg, err := s.ldap.FindByName(ident.Directory)
	if err != nil || len(cfg.Mappings) == 0 {
		return err
	}
	plan := Plan(cfg.Mappings, ident.Groups)
	if plan.AdminManaged {
		s.reconcileAdmin(user, plan.Admin)
	}
	for wsID := range plan.Managed {
		role, want := plan.Roles[wsID]
		member, err := s.workspaces.FindMember(wsID, user.ID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("directory: read membership failed", "workspace", wsID, "user", user.ID, "error", err)
			continue
		}
		switch {
		case member != nil && member.Role == models.WorkspaceRoleOwner:
			// The owner's membership is never changed from a directory.
		case want && member == nil:
			if err := s.workspaces.AddMember(&models.WorkspaceMember{WorkspaceID: wsID, UserID: user.ID, Role: role}); err != nil {
				logger.Warn("directory: add member failed", "workspace", wsID, "user", user.ID, "error", err)
			}
		case want && member.Role != role:
			if err := s.workspaces.UpdateMemberRole(wsID, user.ID, role); err != nil {
				logger.Warn("directory: change role failed", "workspace", wsID, "user", user.ID, "error", err)
			}
		case !want && member != nil:
			if err := s.workspaces.RemoveMember(wsID, user.ID); err != nil {
				logger.Warn("directory: remove member failed", "workspace", wsID, "user", user.ID, "error", err)
			}
		}
	}
	return nil
}

// reconcileAdmin never takes the admin role from the last admin who can sign in.
func (s *Service) reconcileAdmin(user *models.User, want bool) {
	isAdmin := user.Role == models.UserRoleAdmin
	switch {
	case want && !isAdmin:
		user.Role = models.UserRoleAdmin
	case !want && isAdmin:
		if n, err := s.users.CountActiveAdmins(); err != nil || n <= 1 {
			return
		}
		user.Role = models.UserRoleUser
	default:
		return
	}
	if err := s.users.Update(user); err != nil {
		logger.Warn("directory: change platform role failed", "user", user.ID, "error", err)
	}
}

type Access struct {
	// AdminManaged reports whether any mapping governs the admin role at all.
	Admin        bool
	AdminManaged bool
	// Roles holds the highest role granted per workspace; Managed every workspace a mapping names.
	Roles   map[uint]models.WorkspaceRole
	Managed map[uint]bool
}

func Plan(mappings []models.LDAPGroupMapping, groups []string) Access {
	have := normalizeGroups(groups)
	a := Access{Roles: map[uint]models.WorkspaceRole{}, Managed: map[uint]bool{}}
	for _, m := range mappings {
		if m.PlatformAdmin {
			a.AdminManaged = true
		}
		if m.WorkspaceID != nil {
			a.Managed[*m.WorkspaceID] = true
		}
		if !groupMatches(have, m.GroupDN) {
			continue
		}
		if m.PlatformAdmin {
			a.Admin = true
		}
		if m.WorkspaceID != nil && roleRank(m.WorkspaceRole) > roleRank(a.Roles[*m.WorkspaceID]) {
			a.Roles[*m.WorkspaceID] = m.WorkspaceRole
		}
	}
	return a
}

// Owner is not mappable: a workspace has exactly one owner, never managed by a directory.
var MappableRoles = []models.WorkspaceRole{models.WorkspaceRoleAdmin, models.WorkspaceRoleEditor, models.WorkspaceRoleViewer}

func roleRank(r models.WorkspaceRole) int {
	switch r {
	case models.WorkspaceRoleAdmin:
		return 3
	case models.WorkspaceRoleEditor:
		return 2
	case models.WorkspaceRoleViewer:
		return 1
	}
	return 0
}

func normalizeGroups(groups []string) map[string]bool {
	m := make(map[string]bool, len(groups))
	for _, g := range groups {
		if g = strings.ToLower(strings.TrimSpace(g)); g != "" {
			m[g] = true
		}
	}
	return m
}

// groupMatches accepts a full DN or a bare CN and ignores case.
func groupMatches(have map[string]bool, group string) bool {
	g := strings.ToLower(strings.TrimSpace(group))
	if g == "" {
		return false
	}
	if have[g] {
		return true
	}
	if !strings.Contains(g, "=") {
		for dn := range have {
			if cnOf(dn) == g {
				return true
			}
		}
	}
	return false
}

// cnOf is the value of a DN's first RDN: "cn=admins,ou=groups" gives "admins".
func cnOf(dn string) string {
	first, _, _ := strings.Cut(dn, ",")
	if _, v, ok := strings.Cut(first, "="); ok {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(first)
}

// unusablePassword is a bcrypt hash of random bytes so a directory account can never sign in locally.
func unusablePassword() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	hash, _ := bcrypt.GenerateFromPassword(b, bcrypt.DefaultCost)
	return string(hash)
}
