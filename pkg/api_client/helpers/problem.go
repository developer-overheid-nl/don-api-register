package helpers

type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// APIError implementeert error + Problem Details (RFC 7807)
type APIError struct {
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Status        int            `json:"status"`
	Detail        string         `json:"detail"`
	Instance      string         `json:"instance,omitempty"`
	InvalidParams []InvalidParam `json:"invalidParams,omitempty"`
}

func (e APIError) Error() string { return e.Detail }

// Constructor voor 400 Bad Request
func NewBadRequest(detail string, params ...InvalidParam) APIError {
	return APIError{
		Type:          "https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/400",
		Title:         "Bad Request",
		Status:        400,
		Detail:        detail,
		InvalidParams: params,
	}
}

// Constructor voor 404 Not Found
func NewNotFound(detail string, params ...InvalidParam) APIError {
	return APIError{
		Type:          "https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/404",
		Title:         "Not Found",
		Status:        404,
		Detail:        detail,
		InvalidParams: params,
	}
}

func NewInternalServerError(detail string) APIError {
	return APIError{
		Type:   "https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/500",
		Title:  "Internal Server Error",
		Status: 500,
		Detail: detail,
	}
}
