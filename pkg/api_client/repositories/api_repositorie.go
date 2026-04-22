package repositories

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"gorm.io/gorm"
	"sigs.k8s.io/yaml"
)

type ApiRepository interface {
	GetApis(ctx context.Context, page, perPage int, p *models.ApiFiltersParams) ([]models.Api, models.Pagination, error)
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
	ListLintResults(ctx context.Context) ([]models.LintResult, error)
	GetOrganisations(ctx context.Context) ([]models.Organisation, int, error)
	FindOrganisationByURI(ctx context.Context, uri string) (*models.Organisation, error)
	SaveArtifact(ctx context.Context, art *models.ApiArtifact) error
	HasArtifactOfKind(ctx context.Context, apiID, kind string) (bool, error)
	GetOasArtifact(ctx context.Context, apiID, version, format string) (*models.ApiArtifact, error)
	GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error)
	DeleteArtifactsByKind(ctx context.Context, apiID, kind string, keepIDs []string) error
	GetApiFilterCounts(ctx context.Context, p *models.ApiFiltersParams) (*models.ApiFilterCounts, error)
}

type apiRepository struct {
	db *gorm.DB
}

type apiFilterMatcher struct {
	params          *models.ApiFiltersParams
	organisation    string
	ids             map[string]bool
	status          map[string]bool
	oasVersion      map[string]bool
	adrScore        map[int]bool
	adrScoreUnknown bool
	adrScoreInvalid bool
	auth            map[string]bool
	now             time.Time
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

func (r *apiRepository) GetApis(ctx context.Context, page, perPage int, p *models.ApiFiltersParams) ([]models.Api, models.Pagination, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 10
	}
	matcher := compileApiFilters(p)

	var apis []models.Api
	if err := applyApiOrdering(r.db.WithContext(ctx)).
		Preload("Servers").
		Preload("Organisation").
		Find(&apis).Error; err != nil {
		return nil, models.Pagination{}, err
	}

	oasVersions, err := r.loadOriginalOASVersions(ctx, apis)
	if err != nil {
		return nil, models.Pagination{}, err
	}

	filtered := make([]models.Api, 0, len(apis))
	for _, api := range apis {
		if apiMatchesCompiledFilters(api, matcher, "", oasVersions) {
			filtered = append(filtered, api)
		}
	}

	totalRecords := len(filtered)
	totalPages := 0
	if totalRecords > 0 {
		totalPages = int(math.Ceil(float64(totalRecords) / float64(perPage)))
	}
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
	if page > 1 && totalPages > 0 {
		prev := page - 1
		pagination.Previous = &prev
	}

	offset := (page - 1) * perPage
	if offset >= totalRecords {
		return []models.Api{}, pagination, nil
	}

	end := offset + perPage
	if end > totalRecords {
		end = totalRecords
	}

	return filtered[offset:end], pagination, nil
}

func applyApiOrdering(db *gorm.DB) *gorm.DB {
	return db.Order("title")
}

func (r *apiRepository) GetApiFilterCounts(ctx context.Context, p *models.ApiFiltersParams) (*models.ApiFilterCounts, error) {
	matcher := compileApiFilters(p)

	var apis []models.Api
	if err := r.db.WithContext(ctx).
		Preload("Organisation").
		Find(&apis).Error; err != nil {
		return nil, err
	}

	oasVersions, err := r.loadOriginalOASVersions(ctx, apis)
	if err != nil {
		return nil, err
	}

	result := &models.ApiFilterCounts{}
	result.Organisation = countApisByFieldWithFiltersAndLabel(apis, matcher, "organisation", func(api models.Api) string {
		if api.OrganisationID == nil {
			return ""
		}
		return *api.OrganisationID
	}, func(api models.Api) string {
		if api.Organisation == nil {
			if api.OrganisationID == nil {
				return ""
			}
			return *api.OrganisationID
		}
		if strings.TrimSpace(api.Organisation.Label) == "" {
			return api.Organisation.Uri
		}
		return api.Organisation.Label
	}, false, oasVersions)
	result.Status = countApisByFieldWithFilters(apis, matcher, "status", func(api models.Api) string {
		return api.LifecycleStatus(matcher.now)
	}, oasVersions)
	result.OasVersion = countApisByFieldWithFilters(apis, matcher, "oasVersion", func(api models.Api) string {
		if version := apiOpenAPIVersion(api, oasVersions); version != "" {
			return version
		}
		return "unknown"
	}, oasVersions)
	result.AdrScore = countApisByFieldWithFilters(apis, matcher, "adrScore", func(api models.Api) string {
		if api.AdrScore == nil {
			return "unknown"
		}
		return strconv.Itoa(*api.AdrScore)
	}, oasVersions)
	result.Auth = countApisByFieldWithFilters(apis, matcher, "auth", func(api models.Api) string {
		return normalizedAuthValue(api.Auth)
	}, oasVersions)

	return result, nil
}

func (r *apiRepository) loadOriginalOASVersions(ctx context.Context, apis []models.Api) (map[string]string, error) {
	apiIDs := make([]string, 0, len(apis))
	for _, api := range apis {
		if strings.TrimSpace(api.Id) != "" {
			apiIDs = append(apiIDs, api.Id)
		}
	}
	if len(apiIDs) == 0 {
		return map[string]string{}, nil
	}

	var artifacts []models.ApiArtifact
	if err := r.db.WithContext(ctx).
		Where("kind = ? AND source = ? AND api_id IN ?", "oas", "original", apiIDs).
		Order("created_at desc").
		Find(&artifacts).Error; err != nil {
		return nil, err
	}

	versions := make(map[string]string, len(artifacts))
	for _, art := range artifacts {
		if versions[art.ApiID] != "" {
			continue
		}
		if version := parseOpenAPIVersion(art.Data); version != "" {
			versions[art.ApiID] = version
		}
	}
	return versions, nil
}

func parseOpenAPIVersion(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var payload struct {
		OpenAPI string `json:"openapi" yaml:"openapi"`
	}
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.OpenAPI)
}

func apiOpenAPIVersion(api models.Api, oasVersions map[string]string) string {
	if oasVersions == nil {
		return ""
	}
	return strings.TrimSpace(oasVersions[api.Id])
}

func countApisByFieldWithFilters(apis []models.Api, matcher *apiFilterMatcher, exclude string, getValue func(models.Api) string, oasVersions map[string]string) []models.FilterCount {
	return countApisByFieldWithFiltersAndLabel(apis, matcher, exclude, getValue, nil, true, oasVersions)
}

func countApisByFieldWithFiltersAndLabel(
	apis []models.Api,
	matcher *apiFilterMatcher,
	exclude string,
	getValue func(models.Api) string,
	getLabel func(models.Api) string,
	sortByCount bool,
	oasVersions map[string]string,
) []models.FilterCount {
	counts := make(map[string]int)
	labels := make(map[string]string)
	for _, api := range apis {
		if !apiMatchesCompiledFilters(api, matcher, exclude, oasVersions) {
			continue
		}
		if val := strings.TrimSpace(getValue(api)); val != "" {
			counts[val]++
			if getLabel == nil {
				continue
			}
			label := strings.TrimSpace(getLabel(api))
			if label == "" {
				label = val
			}
			if labels[val] == "" {
				labels[val] = label
			}
		}
	}
	result := make([]models.FilterCount, 0, len(counts))
	for val, count := range counts {
		result = append(result, models.FilterCount{
			Value: val,
			Label: labels[val],
			Count: count,
		})
	}
	sortFilterCounts(result, sortByCount)
	return result
}

func sortFilterCounts(counts []models.FilterCount, sortByCount bool) {
	sort.Slice(counts, func(i, j int) bool {
		if sortByCount && counts[i].Count != counts[j].Count {
			return counts[i].Count > counts[j].Count
		}

		iKey := filterCountSortKey(counts[i])
		jKey := filterCountSortKey(counts[j])
		if iKey != jKey {
			return iKey < jKey
		}
		return strings.ToLower(counts[i].Value) < strings.ToLower(counts[j].Value)
	})
}

func filterCountSortKey(count models.FilterCount) string {
	label := strings.TrimSpace(count.Label)
	if label == "" {
		label = count.Value
	}
	return strings.ToLower(label)
}

func compileApiFilters(p *models.ApiFiltersParams) *apiFilterMatcher {
	if p == nil {
		p = &models.ApiFiltersParams{}
	}

	matcher := &apiFilterMatcher{
		params:     p,
		status:     selectedLowerFilterSet(p.Status),
		oasVersion: selectedFilterSet(p.OasVersion, p.Version),
		auth:       selectedFilterSet(normalizeAuthValues(p.Auth)),
		now:        time.Now(),
	}
	if p.Organisation != nil {
		matcher.organisation = strings.TrimSpace(*p.Organisation)
	}
	if p.Ids != nil {
		matcher.ids = selectedFilterSet([]string{*p.Ids})
	}
	matcher.adrScore, matcher.adrScoreUnknown, matcher.adrScoreInvalid = selectedScoreSet(p.AdrScore)

	return matcher
}

func apiMatchesCompiledFilters(api models.Api, matcher *apiFilterMatcher, exclude string, oasVersions map[string]string) bool {
	if matcher == nil || matcher.params == nil {
		return true
	}
	if exclude != "organisation" && matcher.organisation != "" {
		if api.OrganisationID == nil || *api.OrganisationID != matcher.organisation {
			return false
		}
	}
	if len(matcher.ids) > 0 && !matcher.ids[api.Id] {
		return false
	}
	if exclude != "status" && len(matcher.status) > 0 {
		if !matcher.status[api.LifecycleStatus(matcher.now)] {
			return false
		}
	}
	if exclude != "oasVersion" && len(matcher.oasVersion) > 0 {
		version := apiOpenAPIVersion(api, oasVersions)
		if version == "" {
			version = "unknown"
		}
		if !matcher.oasVersion[version] {
			return false
		}
	}
	if exclude != "adrScore" && (len(matcher.adrScore) > 0 || matcher.adrScoreUnknown || matcher.adrScoreInvalid) {
		if matcher.adrScoreInvalid {
			return false
		}
		if api.AdrScore == nil {
			if !matcher.adrScoreUnknown {
				return false
			}
		} else if !matcher.adrScore[*api.AdrScore] {
			return false
		}
	}
	if exclude != "auth" && len(matcher.auth) > 0 {
		if !matcher.auth[normalizedAuthValue(api.Auth)] {
			return false
		}
	}
	return true
}

func selectedFilterSet(groups ...[]string) map[string]bool {
	values := make(map[string]bool)
	for _, group := range groups {
		for _, raw := range group {
			for _, val := range strings.Split(raw, ",") {
				trimmed := strings.TrimSpace(val)
				if trimmed != "" {
					values[trimmed] = true
				}
			}
		}
	}
	return values
}

func selectedLowerFilterSet(groups ...[]string) map[string]bool {
	values := selectedFilterSet(groups...)
	lowered := make(map[string]bool, len(values))
	for val := range values {
		lowered[strings.ToLower(val)] = true
	}
	return lowered
}

func selectedScoreSet(values []string) (map[int]bool, bool, bool) {
	scores := make(map[int]bool)
	unknown := false
	invalid := false
	for _, raw := range values {
		for _, val := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(val)
			if trimmed == "" {
				continue
			}
			if strings.EqualFold(trimmed, "unknown") {
				unknown = true
				continue
			}
			score, err := strconv.Atoi(trimmed)
			if err != nil || score < 0 || score > 100 {
				invalid = true
				continue
			}
			scores[score] = true
		}
	}
	return scores, unknown, invalid
}

func normalizeAuthValues(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		for _, val := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(val)
			if trimmed == "" {
				continue
			}
			normalized = append(normalized, normalizedAuthValue(trimmed))
		}
	}
	return normalized
}

func normalizedAuthValue(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "":
		return "none"
	case "apikey", "api-key", "api key":
		return "api_key"
	case "openidconnect", "openid-connect":
		return "openid"
	default:
		return trimmed
	}
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

func (r *apiRepository) ListLintResults(ctx context.Context) ([]models.LintResult, error) {
	var results []models.LintResult
	err := r.db.WithContext(ctx).
		Preload("Messages").
		Preload("Messages.Infos").
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

func (r *apiRepository) DeleteArtifactsByKind(ctx context.Context, apiID, kind string, keepIDs []string) error {
	if strings.TrimSpace(apiID) == "" || strings.TrimSpace(kind) == "" {
		return fmt.Errorf("apiID en kind zijn verplicht voor verwijderen")
	}
	query := r.db.WithContext(ctx).
		Where("api_id = ? AND kind = ?", apiID, kind)
	if len(keepIDs) > 0 {
		query = query.Where("id NOT IN ?", keepIDs)
	}
	return query.Delete(&models.ApiArtifact{}).Error
}
