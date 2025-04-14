package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"math"
)

type ApiRepository interface {
	GetApis(ctx context.Context, page, perPage int) ([]models.ApiRawData, models.Pagination, error)
	GetApiByID(ctx context.Context, id string) (*models.ApiRawData, error)
}

type apiRepository struct {
	db *sql.DB
}

func NewApiRepository(db *sql.DB) ApiRepository {
	return &apiRepository{db: db}
}

func (r *apiRepository) GetApis(ctx context.Context, page, perPage int) ([]models.ApiRawData, models.Pagination, error) {
	offset := (page - 1) * perPage

	var totalRecords int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM core_api").Scan(&totalRecords); err != nil {
		return nil, models.Pagination{}, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, api_type, service_name, description, api_authentication
		FROM core_api
		ORDER BY id LIMIT $1 OFFSET $2`, perPage, offset)
	if err != nil {
		return nil, models.Pagination{}, err
	}
	defer rows.Close()

	var apis []models.ApiRawData
	for rows.Next() {
		var raw models.ApiRawData
		if err := rows.Scan(&raw.Id, &raw.Type, &raw.Title, &raw.Description, &raw.Auth); err != nil {
			return nil, models.Pagination{}, err
		}
		apis = append(apis, raw)
	}

	totalPages := int(math.Ceil(float64(totalRecords) / float64(perPage)))
	pagination := models.Pagination{
		CurrentPage:    page,
		RecordsPerPage: perPage,
		TotalPages:     totalPages,
		TotalRecords:   totalRecords,
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

func (r *apiRepository) GetApiByID(ctx context.Context, id string) (*models.ApiRawData, error) {
	var data models.ApiRawData

	err := r.db.QueryRowContext(ctx, `
		SELECT id, api_type, service_name, description, api_authentication
		FROM core_api
		WHERE id = $1`, id).
		Scan(&data.Id, &data.Type, &data.Title, &data.Description, &data.Auth)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// enrich fields (repositoryUri, adrScore, organisation)
	if err := r.enrichApi(ctx, &data); err != nil {
		return nil, err
	}

	return &data, nil
}

func (r *apiRepository) enrichApi(ctx context.Context, raw *models.ApiRawData) error {
	// 1. Organisation (label + uri)
	var orgName string
	var contactJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT name, contact
		FROM organization
		WHERE ooid = (SELECT organization_ooid FROM core_api WHERE id = $1)
	`, raw.Id).Scan(&orgName, &contactJSON)

	if err == nil {
		uri := extractFirstURLFromJSON(contactJSON, "internetadressen", "url")
		raw.Organisation = &models.OrganisationInfo{
			Name: orgName,
			Uri:  uri,
		}
	}

	// 2. Repository URL
	var repoURL *string
	_ = r.db.QueryRowContext(ctx, `
		SELECT url
		FROM repository
		WHERE organization_ooid = (SELECT organization_ooid FROM core_api WHERE id = $1)
		LIMIT 1`, raw.Id).Scan(&repoURL)
	raw.RepositoryUri = repoURL

	// 3. Validator ADR-score
	var score *string
	_ = r.db.QueryRowContext(ctx, `
		SELECT rule_passed_count
		FROM validator_report
		WHERE uri LIKE '%' || $1
		ORDER BY generated_at DESC
		LIMIT 1`, raw.Id).Scan(&score)
	raw.AdrScore = score

	return nil
}

// naar een helper class?
func extractFirstURLFromJSON(jsonBytes []byte, arrayField, key string) *string {
	var parsed map[string][]map[string]string
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		return nil
	}

	if list, ok := parsed[arrayField]; ok && len(list) > 0 {
		if url, ok := list[0][key]; ok {
			return &url
		}
	}
	return nil
}
