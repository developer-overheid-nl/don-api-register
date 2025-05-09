/*
 * API register API v1
 *
 * API van het API register (apis.developer.overheid.nl)
 *
 * API version: 1.0.0
 * Contact: developer.overheid@geonovum.nl
 */

package models

type ValidationErrorResponse struct {
	MissingProperties []string `json:"missingProperties,omitempty"`
	Message           string   `json:"message"`
}
