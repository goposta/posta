package repositories

import (
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type OAuthAccountRepository struct {
	db *gorm.DB
}

func NewOAuthAccountRepository(db *gorm.DB) *OAuthAccountRepository {
	return &OAuthAccountRepository{db: db}
}

func (r *OAuthAccountRepository) Create(a *models.OAuthAccount) error {
	return r.db.Create(a).Error
}

func (r *OAuthAccountRepository) FindByProviderAndExternalID(providerID uint, externalID string) (*models.OAuthAccount, error) {
	var a models.OAuthAccount
	if err := r.db.Where("provider_id = ? AND provider_user_id = ?", providerID, externalID).
		First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *OAuthAccountRepository) FindByUserID(userID uint) ([]models.OAuthAccount, error) {
	var accounts []models.OAuthAccount
	if err := r.db.Preload("Provider").Where("user_id = ?", userID).
		Order("created_at DESC").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *OAuthAccountRepository) FindByUserAndProvider(userID, providerID uint) (*models.OAuthAccount, error) {
	var a models.OAuthAccount
	if err := r.db.Where("user_id = ? AND provider_id = ?", userID, providerID).
		First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *OAuthAccountRepository) Update(a *models.OAuthAccount) error {
	return r.db.Save(a).Error
}

func (r *OAuthAccountRepository) Delete(id uint) error {
	return r.db.Delete(&models.OAuthAccount{}, id).Error
}

func (r *OAuthAccountRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.OAuthAccount{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}
