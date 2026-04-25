package repositories

import (
	"strings"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type OAuthProviderRepository struct {
	db *gorm.DB
}

func NewOAuthProviderRepository(db *gorm.DB) *OAuthProviderRepository {
	return &OAuthProviderRepository{db: db}
}

func (r *OAuthProviderRepository) Create(p *models.OAuthProvider) error {
	return r.db.Create(p).Error
}

func (r *OAuthProviderRepository) FindByID(id uint) (*models.OAuthProvider, error) {
	var p models.OAuthProvider
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *OAuthProviderRepository) FindBySlug(slug string) (*models.OAuthProvider, error) {
	var p models.OAuthProvider
	if err := r.db.Where("slug = ? AND enabled = true", slug).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// FindEnabled returns enabled, non-hidden global providers for the login page button list.
func (r *OAuthProviderRepository) FindEnabled() ([]models.OAuthProvider, error) {
	var providers []models.OAuthProvider
	if err := r.db.Where("enabled = true AND hidden = false AND workspace_id IS NULL").
		Order("name ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// FindEnabledForWorkspace returns enabled, non-hidden providers: global + workspace-scoped.
func (r *OAuthProviderRepository) FindEnabledForWorkspace(wsID uint) ([]models.OAuthProvider, error) {
	var providers []models.OAuthProvider
	if err := r.db.Where("enabled = true AND hidden = false AND (workspace_id IS NULL OR workspace_id = ?)", wsID).
		Order("name ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// HasEnabledHidden reports whether any enabled, hidden provider exists.
// Drives the visibility of the "Continue with SSO" entry point on the login page.
func (r *OAuthProviderRepository) HasEnabledHidden() (bool, error) {
	var count int64
	if err := r.db.Model(&models.OAuthProvider{}).
		Where("enabled = true AND hidden = true").Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindEnabledByDomain scans enabled providers (including hidden) whose AllowedDomains CSV
// contains the given domain. Used for email-based SSO discovery.
func (r *OAuthProviderRepository) FindEnabledByDomain(domain string) (*models.OAuthProvider, error) {
	if domain == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var providers []models.OAuthProvider
	if err := r.db.Where("enabled = true AND allowed_domains <> ''").
		Find(&providers).Error; err != nil {
		return nil, err
	}
	target := strings.ToLower(strings.TrimSpace(domain))
	for i := range providers {
		for _, d := range strings.Split(providers[i].AllowedDomains, ",") {
			if strings.ToLower(strings.TrimSpace(d)) == target {
				return &providers[i], nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// FindAll returns all providers (admin).
func (r *OAuthProviderRepository) FindAll() ([]models.OAuthProvider, error) {
	var providers []models.OAuthProvider
	if err := r.db.Order("created_at DESC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

func (r *OAuthProviderRepository) Update(p *models.OAuthProvider) error {
	return r.db.Save(p).Error
}

func (r *OAuthProviderRepository) Delete(id uint) error {
	return r.db.Delete(&models.OAuthProvider{}, id).Error
}
