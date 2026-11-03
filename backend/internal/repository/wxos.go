package repository

import (
	"gorm.io/gorm/clause"
	"time"
	"yingce/backend/internal/model"
)

func (r *Repository) UseWxosLoginNonce(state *model.OAuthState) (bool, error) {
	if err := r.db.Where("provider = ? AND expires_at < ?", "wxos", time.Now().Add(-24*time.Hour)).Delete(&model.OAuthState{}).Error; err != nil {
		return false, err
	}
	result := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "state_hash"}}, DoNothing: true}).Create(state)
	return result.RowsAffected == 1, result.Error
}
