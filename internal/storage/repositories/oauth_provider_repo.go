package repositories

import (
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

// FindEnabled returns all enabled global providers (workspace_id IS NULL).
func (r *OAuthProviderRepository) FindEnabled() ([]models.OAuthProvider, error) {
	var providers []models.OAuthProvider
	if err := r.db.Where("enabled = true AND workspace_id IS NULL").
		Order("name ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// FindEnabledForWorkspace returns enabled providers: global + workspace-scoped.
func (r *OAuthProviderRepository) FindEnabledForWorkspace(wsID uint) ([]models.OAuthProvider, error) {
	var providers []models.OAuthProvider
	if err := r.db.Where("enabled = true AND (workspace_id IS NULL OR workspace_id = ?)", wsID).
		Order("name ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
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
