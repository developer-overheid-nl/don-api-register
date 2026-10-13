package problem

import commonproblem "github.com/developer-overheid-nl/don-register-common/problem"

type InvalidParam = commonproblem.InvalidParam
type ErrorDetail = commonproblem.ErrorDetail

// APIError implementeert error + Problem Details (RFC 9457) met de door de
// NL GOV API Design Rules verplichte velden status, title en detail.
type APIError struct {
	Title  string        `json:"title"`
	Status int           `json:"status"`
	Detail string        `json:"detail"`
	Errors []ErrorDetail `json:"errors,omitempty"`
}

func (e APIError) Error() string { return e.Title }

func New(status int, title string, details ...ErrorDetail) APIError {
	return newAPIError(status, title, "", details)
}

func NewBadRequest(oasUri, detail string, params ...InvalidParam) APIError {
	return newAPIError(400, "Request validation failed", detail,
		commonproblem.ErrorDetailsFromInvalidParams(params, detail, "body", "body", "bad_request"))
}

func NewNotFound(oasUri, detail string, params ...InvalidParam) APIError {
	return newAPIError(404, "Resource Not Found", detail,
		toErrorDetails(params, detail, "path", oasUri, "not_found"))
}

func NewInternalServerError(detail string) APIError {
	return newAPIError(500, "Internal Server Error", detail,
		toErrorDetails(nil, detail, "", "", "internal_error"))
}

func NewForbidden(oasUri, detail string) APIError {
	return newAPIError(403, "Forbidden", detail,
		toErrorDetails(nil, detail, "", "", "forbidden"))
}

// newAPIError vult detail altijd: de expliciete detail, anders de detail van de
// eerste fout, anders de titel.
func newAPIError(status int, title, detail string, details []ErrorDetail) APIError {
	if detail == "" && len(details) > 0 {
		detail = details[0].Detail
	}
	if detail == "" {
		detail = title
	}
	return APIError{
		Title:  title,
		Status: status,
		Detail: detail,
		Errors: details,
	}
}

func toErrorDetails(params []InvalidParam, fallbackDetail, fallbackIn, fallbackLocation, fallbackCode string) []ErrorDetail {
	return commonproblem.ErrorDetailsFromInvalidParams(params, fallbackDetail, fallbackIn, fallbackLocation, fallbackCode)
}
