package repositories

import (
	"context"
	"errors"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"gorm.io/gorm"
	"math"
)

type ApiRepository interface {
	GetApis(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error)
	GetApiByID(ctx context.Context, id string) (*models.Api, error)
	Save(api *models.Api) error
	UpdateApi(ctx context.Context, api models.Api) error
	FindByOasUrl(ctx context.Context, oasUrl string) (*models.Api, error)
	SaveServer(server models.Server) error
	SaveOrganisatie(organisation *models.Organisation) error
}

type apiRepository struct {
	db *gorm.DB
}

func NewApiRepository(db *gorm.DB) ApiRepository {
	return &apiRepository{db: db}
}

func (r *apiRepository) Save(api *models.Api) error {
	oldApi, _ := r.FindByOasUrl(context.Background(), api.OasUri)
	if oldApi == nil {
		return r.db.Create(api).Error
	} else {
		api.Id = oldApi.Id
		return r.db.Save(api).Error
	}
}

func (r *apiRepository) GetApis(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error) {
	offset := (page - 1) * perPage

	var totalRecords int64
	if err := r.db.Model(&models.Api{}).Count(&totalRecords).Error; err != nil {
		return nil, models.Pagination{}, err
	}

	var apis []models.Api
	if err := r.db.Limit(perPage).Preload("Servers").Offset(offset).Order("id").Find(&apis).Error; err != nil {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func (r *apiRepository) UpdateApi(ctx context.Context, api models.Api) error {
	return r.db.WithContext(ctx).Model(&models.Api{}).
		Where("id = ?", api.Id).
		Updates(api).Error
}

func (r *apiRepository) FindByOasUrl(ctx context.Context, oasUrl string) (*models.Api, error) {
	var api models.Api
	if err := r.db.WithContext(ctx).Where("oas_uri = ?", oasUrl).First(&api).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func (r *apiRepository) SaveServer(server models.Server) error {
	return r.db.Save(&server).Error
}
func (r *apiRepository) SaveOrganisatie(organisation *models.Organisation) error {
	return r.db.Save(&organisation).Error

}
