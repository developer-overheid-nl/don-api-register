package models

import "fmt"

type ApiSortField string

type ApiSortOrder string

const (
	ApiSortTitle    ApiSortField = "title"
	ApiSortADRScore ApiSortField = "adrScore"
	ApiSortVersion  ApiSortField = "version"

	ApiSortAscending  ApiSortOrder = "asc"
	ApiSortDescending ApiSortOrder = "desc"
)

type ApiSort struct {
	Field ApiSortField
	Order ApiSortOrder
}

type InvalidApiSortError struct {
	Parameter string
	Value     string
}

func (e InvalidApiSortError) Error() string {
	switch e.Parameter {
	case "sortBy":
		return fmt.Sprintf("sortBy %q is ongeldig; toegestane waarden zijn title, adrScore en version", e.Value)
	case "sortOrder":
		return fmt.Sprintf("sortOrder %q is ongeldig; toegestane waarden zijn asc en desc", e.Value)
	default:
		return fmt.Sprintf("ongeldige sorteerparameter %q", e.Value)
	}
}

func ParseApiSort(sortBy, sortOrder string) (ApiSort, error) {
	if sortBy == "" {
		sortBy = string(ApiSortTitle)
	}
	if sortOrder == "" {
		sortOrder = string(ApiSortAscending)
	}

	field := ApiSortField(sortBy)
	switch field {
	case ApiSortTitle, ApiSortADRScore, ApiSortVersion:
	default:
		return ApiSort{}, InvalidApiSortError{Parameter: "sortBy", Value: sortBy}
	}

	order := ApiSortOrder(sortOrder)
	switch order {
	case ApiSortAscending, ApiSortDescending:
	default:
		return ApiSort{}, InvalidApiSortError{Parameter: "sortOrder", Value: sortOrder}
	}

	return ApiSort{Field: field, Order: order}, nil
}
