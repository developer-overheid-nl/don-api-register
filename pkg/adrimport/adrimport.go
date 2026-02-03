package adrimport

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

type Logger interface {
	Printf(format string, v ...any)
}

type Options struct {
	CSVPath string
	DryRun  bool
	Logger  Logger
}

type Result struct {
	Processed   int
	Inserted    int
	Missing     int
	ParseErrors int
}

type csvRow struct {
	ApiID       string
	URI         string
	Timestamp   time.Time
	Failures    int
	Success     bool
	AdrVersion  string
	FailedRules []failedRule
}

type failedRule struct {
	Code    string `json:"code"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type headerIndex struct {
	apiID      int
	uri        int
	timestamp  int
	failures   int
	isSuccess  int
	adrVersion int
	failed     int
}

func ImportCSV(ctx context.Context, db *gorm.DB, opts Options) (Result, error) {
	if db == nil {
		return Result{}, errors.New("db is nil")
	}
	csvPath := strings.TrimSpace(opts.CSVPath)
	if csvPath == "" {
		return Result{}, errors.New("csv path is empty")
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	file, err := os.Open(csvPath)
	if err != nil {
		return Result{}, fmt.Errorf("failed to open csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	headers, err := reader.Read()
	if err != nil {
		return Result{}, fmt.Errorf("failed to read csv header: %w", err)
	}
	idx, err := mapHeaders(headers)
	if err != nil {
		return Result{}, fmt.Errorf("invalid csv header: %w", err)
	}

	result := Result{}
	line := 1

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line++
		if err != nil {
			logger.Printf("line %d: read error: %v", line, err)
			result.ParseErrors++
			continue
		}
		row, err := parseRow(record, idx)
		if err != nil {
			logger.Printf("line %d: %v", line, err)
			result.ParseErrors++
			continue
		}
		result.Processed++

		api, err := findApiByID(db, row.ApiID)
		if err != nil {
			logger.Printf("line %d: %v", line, err)
			result.ParseErrors++
			continue
		}
		if api == nil {
			result.Missing++
			logger.Printf("line %d: api not found for api_id=%q", line, row.ApiID)
			continue
		}
		if api.Id == "" {
			result.ParseErrors++
			logger.Printf("line %d: api record has empty id for api_id=%q", line, row.ApiID)
			continue
		}

		lint := buildLintResult(api.Id, row)
		if opts.DryRun {
			result.Inserted++
			continue
		}
		if err := saveLintResult(ctx, db, lint); err != nil {
			logger.Printf("line %d: save lint result failed: %v", line, err)
			result.ParseErrors++
			continue
		}
		result.Inserted++
	}

	logger.Printf("done: processed=%d inserted=%d missing=%d parse_errors=%d", result.Processed, result.Inserted, result.Missing, result.ParseErrors)
	return result, nil
}

func mapHeaders(headers []string) (headerIndex, error) {
	idx := map[string]int{}
	for i, h := range headers {
		key := strings.TrimSpace(strings.ToLower(h))
		idx[key] = i
	}
	required := []string{"api_id", "failures", "is_success", "adr_version", "failed_rules"}
	for _, key := range required {
		if _, ok := idx[key]; !ok {
			return headerIndex{}, fmt.Errorf("missing column %q", key)
		}
	}
	tsKey := "timestamp"
	if _, ok := idx[tsKey]; !ok {
		if _, ok := idx["timestamptz"]; ok {
			tsKey = "timestamptz"
		} else {
			return headerIndex{}, fmt.Errorf("missing column %q (or timestamptz)", tsKey)
		}
	}
	uriIdx := -1
	if value, ok := idx["uri"]; ok {
		uriIdx = value
	}
	return headerIndex{
		apiID:      idx["api_id"],
		uri:        uriIdx,
		timestamp:  idx[tsKey],
		failures:   idx["failures"],
		isSuccess:  idx["is_success"],
		adrVersion: idx["adr_version"],
		failed:     idx["failed_rules"],
	}, nil
}

func parseRow(record []string, idx headerIndex) (*csvRow, error) {
	apiID := ""
	if idx.apiID < len(record) {
		apiID = strings.TrimSpace(record[idx.apiID])
	}
	if apiID == "" {
		return nil, fmt.Errorf("missing api_id value")
	}

	uri := ""
	if idx.uri >= 0 && idx.uri < len(record) {
		uri = strings.TrimSpace(record[idx.uri])
	}

	if idx.timestamp >= len(record) {
		return nil, fmt.Errorf("missing timestamp value")
	}
	timestamp, err := parseTimestamp(record[idx.timestamp])
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp %q: %w", record[idx.timestamp], err)
	}

	if idx.failures >= len(record) {
		return nil, fmt.Errorf("missing failures value")
	}
	failures, err := strconv.Atoi(strings.TrimSpace(record[idx.failures]))
	if err != nil {
		return nil, fmt.Errorf("invalid failures %q: %w", record[idx.failures], err)
	}

	if idx.isSuccess >= len(record) {
		return nil, fmt.Errorf("missing is_success value")
	}
	success, err := parseBool(record[idx.isSuccess])
	if err != nil {
		return nil, fmt.Errorf("invalid is_success %q: %w", record[idx.isSuccess], err)
	}

	adrVersion := ""
	if idx.adrVersion < len(record) {
		adrVersion = strings.TrimSpace(record[idx.adrVersion])
	}

	failedRaw := ""
	if idx.failed < len(record) {
		failedRaw = strings.TrimSpace(record[idx.failed])
	}
	failedRules, err := parseFailedRules(failedRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid failed_rules: %w", err)
	}

	return &csvRow{
		ApiID:       apiID,
		URI:         uri,
		Timestamp:   timestamp,
		Failures:    failures,
		Success:     success,
		AdrVersion:  adrVersion,
		FailedRules: failedRules,
	}, nil
}

func parseFailedRules(value string) ([]failedRule, error) {
	v := strings.TrimSpace(value)
	if v == "" || strings.EqualFold(v, "null") {
		return nil, nil
	}
	var rules []failedRule
	if err := json.Unmarshal([]byte(v), &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func parseBool(value string) (bool, error) {
	v := strings.TrimSpace(strings.ToLower(value))
	switch v {
	case "true", "t", "1", "yes", "y":
		return true, nil
	case "false", "f", "0", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func parseTimestamp(value string) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05-07:00",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, v); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp %q", value)
}

func buildLintResult(apiID string, row *csvRow) *models.LintResult {
	return &models.LintResult{
		ID:        uuid.NewString(),
		ApiID:     apiID,
		Successes: row.Success,
		Failures:  row.Failures,
		Warnings:  0,
		CreatedAt: row.Timestamp,
		Messages:  buildLintMessages(row.FailedRules, row.Timestamp),
	}
}

func buildLintMessages(rules []failedRule, createdAt time.Time) []models.LintMessage {
	if len(rules) == 0 {
		return nil
	}
	groups := map[string]*models.LintMessage{}
	for _, rule := range rules {
		code := strings.TrimSpace(rule.Code)
		if code == "" {
			code = "UNKNOWN"
		}
		msg := groups[code]
		if msg == nil {
			msg = &models.LintMessage{
				ID:        uuid.NewString(),
				Code:      code,
				Severity:  "error",
				CreatedAt: createdAt,
				Infos:     []models.LintMessageInfo{},
			}
			groups[code] = msg
		}
		infoMessage := strings.TrimSpace(rule.Message)
		ruleLabel := strings.TrimSpace(rule.Rule)
		if ruleLabel != "" {
			if infoMessage != "" {
				infoMessage = fmt.Sprintf("%s: %s", ruleLabel, infoMessage)
			} else {
				infoMessage = ruleLabel
			}
		}
		msg.Infos = append(msg.Infos, models.LintMessageInfo{
			ID:            uuid.NewString(),
			LintMessageID: msg.ID,
			Message:       infoMessage,
			Path:          "",
		})
	}
	out := make([]models.LintMessage, 0, len(groups))
	for _, msg := range groups {
		out = append(out, *msg)
	}
	return out
}

func findApiByID(db *gorm.DB, apiID string) (*models.Api, error) {
	var api models.Api
	if err := db.Where("id = ?", apiID).First(&api).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func saveLintResult(ctx context.Context, db *gorm.DB, result *models.LintResult) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range result.Messages {
			result.Messages[i].LintResultID = result.ID
		}
		return tx.Create(result).Error
	})
}
