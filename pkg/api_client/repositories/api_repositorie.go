package repositories

import (
	"context"
	"errors"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"gorm.io/gorm"
	"math"
)

type ApiRepository interface {
	GetApis(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error)
	GetApiByID(ctx context.Context, oasUrl string) (*models.Api, error)
	Save(api *models.Api) error
	UpdateApi(ctx context.Context, api models.Api) error
	FindByOasUrl(ctx context.Context, oasUrl string) (*models.Api, error)
	SaveServer(server models.Server) error
	SaveOrganisatie(organisation *models.Organisation) error
	AllApis(ctx context.Context) ([]models.Api, error)
	SaveLintResult(ctx context.Context, result *models.LintResult) error
	GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error)
	GetOrganisations(ctx context.Context) ([]models.Organisation, error)
	FindOrganisationByURI(ctx context.Context, uri string) (*models.Organisation, error)
}

type apiRepository struct {
	db *gorm.DB
}

func NewApiRepository(db *gorm.DB) ApiRepository {
	return &apiRepository{db: db}
}

func (r *apiRepository) Save(api *models.Api) error {
	oldApi, err := r.FindByOasUrl(context.Background(), api.OasUri)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if oldApi != nil {
		return errors.New("api bestaat al")
	}
	return r.db.Create(api).Error
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

func (r *apiRepository) AllApis(ctx context.Context) ([]models.Api, error) {
	var apis []models.Api
	if err := r.db.WithContext(ctx).Find(&apis).Error; err != nil {
		return nil, err
	}
	return apis, nil
}

func (r *apiRepository) SaveLintResult(ctx context.Context, result *models.LintResult) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(result.Messages) > 0 {
			for i := range result.Messages {
				result.Messages[i].LintResultID = result.ID
			}
		}
		fmt.Printf("%+v\n", result)
		if err := tx.Create(result).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *apiRepository) GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error) {
	var results []models.LintResult
	err := r.db.WithContext(ctx).Preload("Messages").
		Preload("Messages.Infos").
		Where("api_id = ?", apiID).
		Order("created_at desc").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *apiRepository) GetOrganisations(ctx context.Context) ([]models.Organisation, error) {
	var organisations []models.Organisation
	if err := r.db.WithContext(ctx).Order("label asc").Find(&organisations).Error; err != nil {
		return nil, err
	}
	return organisations, nil
}

func (r *apiRepository) FindOrganisationByURI(ctx context.Context, uri string) (*models.Organisation, error) {
	var org models.Organisation
	if err := r.db.WithContext(ctx).Where("uri = ?", uri).First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &org, nil
}
