package problem

type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ErrorDetail struct {
	In       string `json:"in"`
	Location string `json:"location"`
	Code     string `json:"code"`
	Detail   string `json:"detail"`
}

// APIError implementeert error + Problem Details (RFC 7807)
type APIError struct {
	Title  string        `json:"title"`
	Status int           `json:"status"`
	Errors []ErrorDetail `json:"errors,omitempty"`
}

func (e APIError) Error() string { return e.Title }

func NewBadRequest(oasUri, detail string, params ...InvalidParam) APIError {
	return APIError{
		Title:  "Request validation failed",
		Status: 400,
		Errors: toErrorDetails(params, detail, "body", "body", "bad_request"),
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
	if len(params) == 0 {
		if fallbackDetail == "" {
			return nil
		}
		return []ErrorDetail{{
			In:       fallbackIn,
			Location: fallbackLocation,
			Code:     fallbackCode,
			Detail:   fallbackDetail,
		}}
	}
	out := make([]ErrorDetail, 0, len(params))
	for _, p := range params {
		out = append(out, ErrorDetail{
			In:       "body",
			Location: p.Name,
			Code:     p.Name,
			Detail:   p.Reason,
		})
	}
	return out
}
