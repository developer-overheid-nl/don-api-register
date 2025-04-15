package repositories

import (
	"context"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"gorm.io/gorm"
	"math"
)

type ApiRepository interface {
	GetApis(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error)
	GetApiByID(ctx context.Context, id string) (*models.Api, error)
	Save(api *models.Api) error
}

type apiRepository struct {
	db *gorm.DB
}

func NewApiRepository(db *gorm.DB) ApiRepository {
	return &apiRepository{db: db}
}

func (r *apiRepository) Save(api *models.Api) error {
	return r.db.Create(api).Error
}

func (r *apiRepository) GetApis(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error) {
	offset := (page - 1) * perPage

	var totalRecords int64
	if err := r.db.Model(&models.Api{}).Count(&totalRecords).Error; err != nil {
		return nil, models.Pagination{}, err
	}

	var apis []models.Api
	if err := r.db.Limit(perPage).Offset(offset).Order("id").Find(&apis).Error; err != nil {
		return nil, models.Pagination{}, err
	}

	totalPages := int(math.Ceil(float64(totalRecords) / float64(perPage)))
	pagination := models.Pagination{
		CurrentPage:    page,
		RecordsPerPage: perPage,
		TotalPages:     totalPages,
		TotalRecords:   int(totalRecords),
	}

	if page < totalPages {
		next := page + 1
		pagination.Next = &next
	}
	if page > 1 {
		prev := page - 1
		pagination.Previous = &prev
	}

	return apis, pagination, nil
}

func (r *apiRepository) GetApiByID(ctx context.Context, id string) (*models.Api, error) {
	var api models.Api
	if err := r.db.First(&api, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &api, nil
}
