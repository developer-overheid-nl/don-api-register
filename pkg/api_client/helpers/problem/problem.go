package problem

import (
	"net/http"

	commonproblem "github.com/developer-overheid-nl/don-register-common/problem"
)

type InvalidParam = commonproblem.InvalidParam
type ErrorDetail = commonproblem.ErrorDetail

// APIError implementeert error + Problem Details (RFC 9457) met de door de
// NL GOV API Design Rules verplichte velden status, title en detail.
type APIError = commonproblem.Problem

func New(status int, title string, details ...ErrorDetail) APIError {
	return commonproblem.New(status, title, details...)
}

func NewBadRequest(oasUri, detail string, params ...InvalidParam) APIError {
	return commonproblem.NewWithDetail(http.StatusBadRequest, "Request validation failed", detail,
		commonproblem.ErrorDetailsFromInvalidParams(params, detail, "body", "body", "bad_request")...)
}

func NewNotFound(oasUri, detail string, params ...InvalidParam) APIError {
	return commonproblem.NewWithDetail(http.StatusNotFound, "Resource Not Found", detail,
		toErrorDetails(params, detail, "path", oasUri, "not_found")...)
}

func NewInternalServerError(detail string) APIError {
	return commonproblem.NewWithDetail(http.StatusInternalServerError, "Internal Server Error", detail,
		toErrorDetails(nil, detail, "", "", "internal_error")...)
}

func NewForbidden(oasUri, detail string) APIError {
	return commonproblem.NewWithDetail(http.StatusForbidden, "Forbidden", detail,
		toErrorDetails(nil, detail, "", "", "forbidden")...)
}

func toErrorDetails(params []InvalidParam, fallbackDetail, fallbackIn, fallbackLocation, fallbackCode string) []ErrorDetail {
	return commonproblem.ErrorDetailsFromInvalidParams(params, fallbackDetail, fallbackIn, fallbackLocation, fallbackCode)
}
