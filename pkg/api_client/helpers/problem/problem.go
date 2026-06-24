package problem

import commonproblem "github.com/developer-overheid-nl/don-register-common/problem"

type InvalidParam = commonproblem.InvalidParam
type ErrorDetail = commonproblem.ErrorDetail

// APIError implementeert error + Problem Details (RFC 7807)
type APIError = commonproblem.Problem

func NewBadRequest(oasUri, detail string, params ...InvalidParam) APIError {
	return APIError{
		Title:  "Request validation failed",
		Status: 400,
		Errors: commonproblem.ErrorDetailsFromInvalidParams(params, detail, "body", "body", "bad_request"),
	}
}

func NewNotFound(oasUri, detail string, params ...InvalidParam) APIError {
	return APIError{
		Title:  "Resource Not Found",
		Status: 404,
		Errors: toErrorDetails(params, detail, "path", oasUri, "not_found"),
	}
}

func NewInternalServerError(detail string) APIError {
	return APIError{
		Title:  "Internal Server Error",
		Status: 500,
		Errors: toErrorDetails(nil, detail, "", "", "internal_error"),
	}
}

func NewForbidden(oasUri, detail string) APIError {
	return APIError{
		Title:  "Forbidden",
		Status: 403,
		Errors: toErrorDetails(nil, detail, "", "", "forbidden"),
	}
}

func toErrorDetails(params []InvalidParam, fallbackDetail, fallbackIn, fallbackLocation, fallbackCode string) []ErrorDetail {
	return commonproblem.ErrorDetailsFromInvalidParams(params, fallbackDetail, fallbackIn, fallbackLocation, fallbackCode)
}
