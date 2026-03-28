package repositories

import (
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type WorkspaceSSORepository struct {
	db *gorm.DB
}

func NewWorkspaceSSORepository(db *gorm.DB) *WorkspaceSSORepository {
	return &WorkspaceSSORepository{db: db}
}

func (r *WorkspaceSSORepository) FindByWorkspaceID(wsID uint) (*models.WorkspaceSSOConfig, error) {
	var c models.WorkspaceSSOConfig
	if err := r.db.Preload("Provider").Where("workspace_id = ?", wsID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *WorkspaceSSORepository) Upsert(c *models.WorkspaceSSOConfig) error {
	var existing models.WorkspaceSSOConfig
	if err := r.db.Where("workspace_id = ?", c.WorkspaceID).First(&existing).Error; err != nil {
		return r.db.Create(c).Error
	}
	existing.ProviderID = c.ProviderID
	existing.EnforceSSO = c.EnforceSSO
	existing.AutoProvision = c.AutoProvision
	existing.AllowedDomains = c.AllowedDomains
	return r.db.Save(&existing).Error
}

func (r *WorkspaceSSORepository) Delete(wsID uint) error {
	return r.db.Where("workspace_id = ?", wsID).Delete(&models.WorkspaceSSOConfig{}).Error
}
