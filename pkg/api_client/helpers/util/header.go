package util

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

func SetPaginationHeaders(r *http.Request, setHeader func(key, val string), p models.Pagination) {
	setHeader("Total-Count", strconv.Itoa(p.TotalRecords))
	setHeader("Total-Pages", strconv.Itoa(p.TotalPages))
	setHeader("Per-Page", strconv.Itoa(p.RecordsPerPage))
	setHeader("Current-Page", strconv.Itoa(p.CurrentPage))

	links := buildLinkHeader(r, p)
	if links != "" {
		setHeader("Link", links)
	}
}

func buildLinkHeader(r *http.Request, p models.Pagination) string {
	if p.TotalPages == 0 {
		return ""
	}

	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host

	makeURL := func(page int) string {
		u := *r.URL
		q := cloneValues(u.Query())
		q.Set("page", strconv.Itoa(page))
		q.Set("perPage", strconv.Itoa(p.RecordsPerPage))
		u.RawQuery = q.Encode()
		return fmt.Sprintf("%s://%s%s", scheme, host, u.RequestURI())
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("<%s>; rel=\"first\"", makeURL(1)))
	if p.Previous != nil {
		parts = append(parts, fmt.Sprintf("<%s>; rel=\"prev\"", makeURL(*p.Previous)))
	}
	parts = append(parts, fmt.Sprintf("<%s>; rel=\"self\"", makeURL(p.CurrentPage)))
	if p.Next != nil {
		parts = append(parts, fmt.Sprintf("<%s>; rel=\"next\"", makeURL(*p.Next)))
	}
	parts = append(parts, fmt.Sprintf("<%s>; rel=\"last\"", makeURL(p.TotalPages)))

	return strings.Join(parts, ", ")
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, arr := range v {
		cp := make([]string, len(arr))
		copy(cp, arr)
		out[k] = cp
	}
	return out
}
