package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// AdoptionRepository provides methods for querying ADR adoption statistics.
type AdoptionRepository interface {
	GetSummary(ctx context.Context, params AdoptionQueryParams) (SummaryResult, int, error)
	GetRules(ctx context.Context, params AdoptionQueryParams) ([]RuleRow, int, error)
	GetTimeline(ctx context.Context, params TimelineQueryParams) ([]TimelineRow, error)
	GetApis(ctx context.Context, params ApisQueryParams) ([]ApiRow, int, error)
}

type AdoptionQueryParams struct {
	AdrVersion   string
	StartDate    time.Time
	EndDate      time.Time
	ApiIds       []string
	Organisation *string
	RuleCodes    []string
	Severity     *string
}

type TimelineQueryParams struct {
	AdoptionQueryParams
	Granularity string
}

type ApisQueryParams struct {
	AdoptionQueryParams
	Compliant *bool
	Page      int
	PerPage   int
}

type SummaryResult struct {
	TotalApis     int
	CompliantApis int
	AdoptionRate  float64
}

type RuleRow struct {
	Code          string
	Severity      string
	ViolatingApis int
	TotalApis     int
}

type TimelineRow struct {
	RuleCode      string
	Period        string
	TotalApis     int
	CompliantApis int
}

type ApiRow struct {
	ApiId         string
	ApiTitle      string
	Organisation  string
	IsCompliant   bool
	TotalFailures int
	TotalWarnings int
	ViolatedRules pq.StringArray `gorm:"type:text[]"`
	LastLintDate  time.Time
}

type adoptionRepository struct {
	db *gorm.DB
}

func NewAdoptionRepository(db *gorm.DB) AdoptionRepository {
	return &adoptionRepository{db: db}
}

// latestResultsCTE builds the common CTE that finds the most recent lint result
// per API on or before endDate for a given adrVersion, with optional filters.
func latestResultsCTE(params AdoptionQueryParams, selectCols string) (string, []interface{}) {
	var args []interface{}

	where := "lr.created_at <= ? AND lr.adr_version = ?"
	args = append(args, params.EndDate, params.AdrVersion)

	if len(params.ApiIds) > 0 {
		placeholders := make([]string, len(params.ApiIds))
		for i, id := range params.ApiIds {
			placeholders[i] = "?"
			args = append(args, id)
		}
		where += " AND lr.api_id IN (" + strings.Join(placeholders, ",") + ")"
	}

	if params.Organisation != nil && strings.TrimSpace(*params.Organisation) != "" {
		where += " AND lr.api_id IN (SELECT id FROM apis WHERE organisation_id = ?)"
		args = append(args, strings.TrimSpace(*params.Organisation))
	}

	cte := fmt.Sprintf(`WITH latest_results AS (
    SELECT DISTINCT ON (lr.api_id) %s
    FROM lint_results lr
    WHERE %s
    ORDER BY lr.api_id, lr.created_at DESC
)`, selectCols, where)

	return cte, args
}

func (r *adoptionRepository) GetSummary(ctx context.Context, params AdoptionQueryParams) (SummaryResult, int, error) {
	cte, args := latestResultsCTE(params, "lr.api_id, lr.successes")

	query := cte + `
SELECT
    COUNT(*) AS total_apis,
    COUNT(*) FILTER (WHERE successes = true) AS compliant_apis,
    COALESCE(ROUND(
        (COUNT(*) FILTER (WHERE successes = true)::numeric / NULLIF(COUNT(*), 0)) * 100,
        1
    ), 0) AS adoption_rate
FROM latest_results`

	var result SummaryResult
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&result).Error; err != nil {
		return SummaryResult{}, 0, fmt.Errorf("summary query failed: %w", err)
	}

	// Count total lint runs in the period
	runsQuery := `SELECT COUNT(*) FROM lint_results WHERE created_at BETWEEN ? AND ? AND adr_version = ?`
	runsArgs := []interface{}{params.StartDate, params.EndDate, params.AdrVersion}

	if len(params.ApiIds) > 0 {
		placeholders := make([]string, len(params.ApiIds))
		for i, id := range params.ApiIds {
			placeholders[i] = "?"
			runsArgs = append(runsArgs, id)
		}
		runsQuery += " AND api_id IN (" + strings.Join(placeholders, ",") + ")"
	}
	if params.Organisation != nil && strings.TrimSpace(*params.Organisation) != "" {
		runsQuery += " AND api_id IN (SELECT id FROM apis WHERE organisation_id = ?)"
		runsArgs = append(runsArgs, strings.TrimSpace(*params.Organisation))
	}

	var totalRuns int
	if err := r.db.WithContext(ctx).Raw(runsQuery, runsArgs...).Scan(&totalRuns).Error; err != nil {
		return SummaryResult{}, 0, fmt.Errorf("lint runs count query failed: %w", err)
	}

	return result, totalRuns, nil
}

func (r *adoptionRepository) GetRules(ctx context.Context, params AdoptionQueryParams) ([]RuleRow, int, error) {
	cte, args := latestResultsCTE(params, "lr.id AS lint_result_id, lr.api_id")

	// Build optional WHERE filters for violations
	violationsWhere := ""
	if len(params.RuleCodes) > 0 {
		placeholders := make([]string, len(params.RuleCodes))
		for i, code := range params.RuleCodes {
			placeholders[i] = "?"
			args = append(args, code)
		}
		violationsWhere += " AND lm.code IN (" + strings.Join(placeholders, ",") + ")"
	}
	if params.Severity != nil && strings.TrimSpace(*params.Severity) != "" {
		violationsWhere += " AND lm.severity = ?"
		args = append(args, strings.TrimSpace(*params.Severity))
	}

	// The total_apis is included in each row via CROSS JOIN
	query := cte + fmt.Sprintf(`,
total_count AS (
    SELECT COUNT(*) AS total_apis FROM latest_results
),
violations_per_rule AS (
    SELECT
        lm.code,
        lm.severity,
        COUNT(DISTINCT lr.api_id) AS violating_apis
    FROM latest_results lr
    JOIN lint_messages lm ON lm.lint_result_id = lr.lint_result_id
    WHERE 1=1%s
    GROUP BY lm.code, lm.severity
)
SELECT
    v.code,
    v.severity,
    v.violating_apis,
    t.total_apis
FROM violations_per_rule v
CROSS JOIN total_count t
ORDER BY v.code`, violationsWhere)

	var rows []RuleRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("rules query failed: %w", err)
	}

	// Extract totalApis from first row if available, otherwise query separately
	totalApis := 0
	if len(rows) > 0 {
		totalApis = rows[0].TotalApis
	} else {
		// No violations found, but we still need total count
		baseCTE, baseArgs := latestResultsCTE(params, "lr.api_id")
		countQuery := baseCTE + ` SELECT COUNT(*) FROM latest_results`
		if err := r.db.WithContext(ctx).Raw(countQuery, baseArgs...).Scan(&totalApis).Error; err != nil {
			return nil, 0, fmt.Errorf("total apis count failed: %w", err)
		}
	}

	return rows, totalApis, nil
}

func (r *adoptionRepository) GetTimeline(ctx context.Context, params TimelineQueryParams) ([]TimelineRow, error) {
	// Determine which rule codes to use
	ruleCodes := params.RuleCodes
	if len(ruleCodes) == 0 {
		codes, err := r.getAllRuleCodes(ctx, params.AdoptionQueryParams)
		if err != nil {
			return nil, err
		}
		ruleCodes = codes
	}

	if len(ruleCodes) == 0 {
		return nil, nil
	}

	// Validate granularity (used as literal in SQL, so must be whitelisted)
	granularity := params.Granularity
	if granularity != "day" && granularity != "week" && granularity != "month" {
		granularity = "month"
	}

	periodFormat := map[string]string{
		"day":   "YYYY-MM-DD",
		"week":  `IYYY-"W"IW`,
		"month": "YYYY-MM",
	}

	periodEndExpr := map[string]string{
		"day":   "ds.period_start",
		"week":  "ds.period_start + interval '6 days'",
		"month": "(ds.period_start + interval '1 month' - interval '1 day')::date",
	}

	// Build base filter for lint_results subqueries
	baseWhere := "adr_version = ?"
	var baseArgs []interface{}
	baseArgs = append(baseArgs, params.AdrVersion)

	if len(params.ApiIds) > 0 {
		placeholders := make([]string, len(params.ApiIds))
		for i, id := range params.ApiIds {
			placeholders[i] = "?"
			baseArgs = append(baseArgs, id)
		}
		baseWhere += " AND api_id IN (" + strings.Join(placeholders, ",") + ")"
	}
	if params.Organisation != nil && strings.TrimSpace(*params.Organisation) != "" {
		baseWhere += " AND api_id IN (SELECT id FROM apis WHERE organisation_id = ?)"
		baseArgs = append(baseArgs, strings.TrimSpace(*params.Organisation))
	}

	// Build rules placeholders
	ruleCodePlaceholders := make([]string, len(ruleCodes))
	var ruleArgs []interface{}
	for i, code := range ruleCodes {
		ruleCodePlaceholders[i] = "?"
		ruleArgs = append(ruleArgs, code)
	}

	query := fmt.Sprintf(`WITH date_series AS (
    SELECT generate_series(
        date_trunc('%s', ?::date),
        date_trunc('%s', ?::date),
        '1 %s'::interval
    )::date AS period_start
),
rules AS (
    SELECT unnest(ARRAY[%s]::text[]) AS code
),
period_rules AS (
    SELECT
        ds.period_start,
        r.code AS rule_code,
        %s AS period_end
    FROM date_series ds
    CROSS JOIN rules r
)
SELECT
    pr.rule_code,
    TO_CHAR(pr.period_start, '%s') AS period,
    (
        SELECT COUNT(DISTINCT api_id)
        FROM lint_results
        WHERE created_at <= pr.period_end AND %s
    ) AS total_apis,
    (
        SELECT COUNT(DISTINCT sub.api_id)
        FROM (
            SELECT DISTINCT ON (api_id) id, api_id
            FROM lint_results
            WHERE created_at <= pr.period_end AND %s
            ORDER BY api_id, created_at DESC
        ) sub
        WHERE NOT EXISTS (
            SELECT 1 FROM lint_messages lm
            WHERE lm.lint_result_id = sub.id
              AND lm.code = pr.rule_code
        )
    ) AS compliant_apis
FROM period_rules pr
ORDER BY pr.rule_code, pr.period_start`,
		granularity, granularity, granularity,
		strings.Join(ruleCodePlaceholders, ","),
		periodEndExpr[granularity],
		periodFormat[granularity],
		baseWhere,
		baseWhere,
	)

	// Args order: startDate, endDate, ruleArgs, baseArgs (total_apis), baseArgs (compliant_apis)
	var args []interface{}
	args = append(args, params.StartDate, params.EndDate)
	args = append(args, ruleArgs...)
	args = append(args, baseArgs...)
	args = append(args, baseArgs...)

	var rows []TimelineRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("timeline query failed: %w", err)
	}

	return rows, nil
}

func (r *adoptionRepository) getAllRuleCodes(ctx context.Context, params AdoptionQueryParams) ([]string, error) {
	cte, args := latestResultsCTE(params, "lr.id AS lint_result_id, lr.api_id")

	query := cte + `
SELECT DISTINCT lm.code
FROM latest_results lr
JOIN lint_messages lm ON lm.lint_result_id = lr.lint_result_id
ORDER BY lm.code`

	var codes []string
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&codes).Error; err != nil {
		return nil, fmt.Errorf("rule codes query failed: %w", err)
	}
	return codes, nil
}

func (r *adoptionRepository) GetApis(ctx context.Context, params ApisQueryParams) ([]ApiRow, int, error) {
	selectCols := "lr.id AS lint_result_id, lr.api_id, lr.successes, lr.failures, lr.warnings, lr.created_at"
	cte, cteArgs := latestResultsCTE(params.AdoptionQueryParams, selectCols)

	baseQuery := cte + `,
api_violations AS (
    SELECT
        lr.api_id,
        ARRAY_AGG(DISTINCT lm.code ORDER BY lm.code) AS violated_rules
    FROM latest_results lr
    JOIN lint_messages lm ON lm.lint_result_id = lr.lint_result_id
    WHERE lm.severity = 'error'
    GROUP BY lr.api_id
)
SELECT
    lr.api_id,
    a.title AS api_title,
    COALESCE(o.label, '') AS organisation,
    lr.successes AS is_compliant,
    lr.failures AS total_failures,
    lr.warnings AS total_warnings,
    COALESCE(av.violated_rules, ARRAY[]::text[]) AS violated_rules,
    lr.created_at AS last_lint_date
FROM latest_results lr
JOIN apis a ON a.id = lr.api_id
LEFT JOIN organisations o ON o.uri = a.organisation_id
LEFT JOIN api_violations av ON av.api_id = lr.api_id`

	// Build optional WHERE clauses
	var whereClauses []string
	var filterArgs []interface{}
	if params.Compliant != nil {
		whereClauses = append(whereClauses, "lr.successes = ?")
		filterArgs = append(filterArgs, *params.Compliant)
	}
	if len(params.RuleCodes) > 0 {
		placeholders := make([]string, len(params.RuleCodes))
		for i, code := range params.RuleCodes {
			placeholders[i] = "?"
			filterArgs = append(filterArgs, code)
		}
		whereClauses = append(whereClauses, "av.violated_rules && ARRAY["+strings.Join(placeholders, ",")+"]::text[]")
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count query: use CTE + count wrapper
	allArgs := append(cteArgs, filterArgs...)
	countQuery := cte + `,
api_violations AS (
    SELECT
        lr.api_id,
        ARRAY_AGG(DISTINCT lm.code ORDER BY lm.code) AS violated_rules
    FROM latest_results lr
    JOIN lint_messages lm ON lm.lint_result_id = lr.lint_result_id
    WHERE lm.severity = 'error'
    GROUP BY lr.api_id
)
SELECT COUNT(*)
FROM latest_results lr
LEFT JOIN api_violations av ON av.api_id = lr.api_id` + whereClause

	var totalCount int
	if err := r.db.WithContext(ctx).Raw(countQuery, allArgs...).Scan(&totalCount).Error; err != nil {
		return nil, 0, fmt.Errorf("apis count query failed: %w", err)
	}

	// Data query with pagination
	offset := (params.Page - 1) * params.PerPage
	dataQuery := baseQuery + whereClause + fmt.Sprintf(" ORDER BY a.title LIMIT %d OFFSET %d", params.PerPage, offset)
	dataArgs := append(cteArgs, filterArgs...)

	var rows []ApiRow
	if err := r.db.WithContext(ctx).Raw(dataQuery, dataArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("apis query failed: %w", err)
	}

	for i := range rows {
		if rows[i].ViolatedRules == nil {
			rows[i].ViolatedRules = pq.StringArray{}
		}
	}

	return rows, totalCount, nil
}
