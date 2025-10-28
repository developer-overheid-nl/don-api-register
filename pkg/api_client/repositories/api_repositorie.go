package repositories

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"gorm.io/gorm"
)

type ApiRepository interface {
	GetApis(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error)
	SearchApis(ctx context.Context, page, perPage int, organisation *string, query string) ([]models.Api, models.Pagination, error)
	GetApiByID(ctx context.Context, oasUrl string) (*models.Api, error)
	Save(api *models.Api) error
	UpdateApi(ctx context.Context, api models.Api) error
	FindByOasUrl(ctx context.Context, oasUrl string) (*models.Api, error)
	SaveServer(server models.Server) error
	SaveOrganisatie(organisation *models.Organisation) error
	AllApis(ctx context.Context) ([]models.Api, error)
	SaveLintResult(ctx context.Context, result *models.LintResult) error
	GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error)
	GetOrganisations(ctx context.Context) ([]models.Organisation, int, error)
	FindOrganisationByURI(ctx context.Context, uri string) (*models.Organisation, error)
	SaveArtifact(ctx context.Context, art *models.ApiArtifact) error
	HasArtifactOfKind(ctx context.Context, apiID, kind string) (bool, error)
	GetOasArtifact(ctx context.Context, apiID, version, format string) (*models.ApiArtifact, error)
	GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error)
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

func (r *apiRepository) GetApis(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
	offset := (page - 1) * perPage

	db := r.db
	if organisation != nil && strings.TrimSpace(*organisation) != "" {
		db = db.Where("organisation_id = ?", strings.TrimSpace(*organisation))
	}
	if ids != nil {
		idsSlice := strings.Split(*ids, ",")
		for i := range idsSlice {
			idsSlice[i] = strings.TrimSpace(idsSlice[i])
		}
		db = db.Where("id IN ?", idsSlice)
	}

	var totalRecords int64
	if err := db.Model(&models.Api{}).Count(&totalRecords).Error; err != nil {
		return nil, models.Pagination{}, err
	}

	var apis []models.Api
	if err := db.Limit(perPage).Preload("Servers").Preload("Organisation").Offset(offset).Order("title").Find(&apis).Error; err != nil {
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

func (r *apiRepository) SearchApis(ctx context.Context, page, perPage int, organisation *string, query string) ([]models.Api, models.Pagination, error) {
	trimmed := strings.TrimSpace(query)
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 10
	}
	if trimmed == "" {
		return []models.Api{}, models.Pagination{
			CurrentPage:    page,
			RecordsPerPage: perPage,
		}, nil
	}

	base := r.db.WithContext(ctx)
	if organisation != nil && strings.TrimSpace(*organisation) != "" {
		base = base.Where("organisation_id = ?", strings.TrimSpace(*organisation))
	}
	var pattern string
	if trimmed != "" {
		pattern = fmt.Sprintf("%%%s%%", strings.ToLower(trimmed))
		base = base.Where("LOWER(title) LIKE ?", pattern)
	}

	var totalRecords int64
	if err := base.Model(&models.Api{}).Count(&totalRecords).Error; err != nil {
		return nil, models.Pagination{}, err
	}

	queryDB := r.db.WithContext(ctx)
	if organisation != nil && strings.TrimSpace(*organisation) != "" {
		queryDB = queryDB.Where("organisation_id = ?", strings.TrimSpace(*organisation))
	}
	if pattern != "" {
		queryDB = queryDB.Where("LOWER(title) LIKE ?", pattern)
	}

	var apis []models.Api
	if err := queryDB.
		Preload("Servers").
		Preload("Organisation").
		Order("title").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&apis).Error; err != nil {
		return nil, models.Pagination{}, err
	}

	totalPages := 0
	if totalRecords > 0 {
		totalPages = int(math.Ceil(float64(totalRecords) / float64(perPage)))
	}
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
	if page > 1 && totalPages > 0 {
		prev := page - 1
		pagination.Previous = &prev
	}

	return apis, pagination, nil
}

func (r *apiRepository) GetApiByID(ctx context.Context, id string) (*models.Api, error) {
	var api models.Api
	if err := r.db.Preload("Servers").Preload("Organisation").First(&api, "id = ?", id).Error; err != nil {
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
	if err := r.db.WithContext(ctx).Preload("Organisation").Preload("Servers").Where("oas_uri = ?", oasUrl).First(&api).Error; err != nil {
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

func (r *apiRepository) GetOrganisations(ctx context.Context) ([]models.Organisation, int, error) {
	var organisations []models.Organisation
	var total int64
	db := r.db.WithContext(ctx)
	if err := db.Model(&models.Organisation{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("label asc").Find(&organisations).Error; err != nil {
		return nil, 0, err
	}
	return organisations, int(total), nil
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

func (r *apiRepository) SaveArtifact(ctx context.Context, art *models.ApiArtifact) error {
	return r.db.WithContext(ctx).Create(art).Error
}

func (r *apiRepository) HasArtifactOfKind(ctx context.Context, apiID, kind string) (bool, error) {
	if strings.TrimSpace(apiID) == "" || strings.TrimSpace(kind) == "" {
		return false, fmt.Errorf("apiID en kind zijn verplicht")
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.ApiArtifact{}).
		Where("api_id = ? AND kind = ?", apiID, kind).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *apiRepository) GetOasArtifact(ctx context.Context, apiID, version, format string) (*models.ApiArtifact, error) {
	if strings.TrimSpace(apiID) == "" || strings.TrimSpace(version) == "" || strings.TrimSpace(format) == "" {
		return nil, fmt.Errorf("apiID, version en format zijn verplicht")
	}
	var art models.ApiArtifact
	query := r.db.WithContext(ctx).
		Where("api_id = ? AND kind = ? AND version = ? AND format = ?", apiID, "oas", version, strings.ToLower(format)).
		Order("CASE WHEN source = 'original' THEN 0 ELSE 1 END").
		Order("created_at desc")
	if err := query.First(&art).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &art, nil
}

func (r *apiRepository) GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
	var a models.ApiArtifact
	if err := r.db.WithContext(ctx).
		Where("api_id = ? AND kind = ?", apiID, kind).
		Order("created_at desc").
		First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}
