package util

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

func LoadOASVersion(path string) (version string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("could not open OAS file: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("could not close OAS file: %w", closeErr)
		}
	}()

	var oas models.OpenAPIInfo
	if err := json.NewDecoder(f).Decode(&oas); err != nil {
		return "", fmt.Errorf("could not parse OAS: %w", err)
	}

	if oas.Info.Version == "" {
		return "", fmt.Errorf("version missing from OAS")
	}

	return oas.Info.Version, nil
}
