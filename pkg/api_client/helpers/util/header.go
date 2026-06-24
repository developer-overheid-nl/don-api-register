package util

import (
	"net/http"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	commonpagination "github.com/developer-overheid-nl/don-register-common/pagination"
)

func SetPaginationHeaders(r *http.Request, setHeader func(key, val string), p models.Pagination) {
	commonpagination.SetHeaders(r, setHeader, p)
}
