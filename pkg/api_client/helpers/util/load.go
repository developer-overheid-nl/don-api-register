package util

import (
	"encoding/json"
	"fmt"
	"os"
)

type OpenAPIInfo struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
}

func LoadOASVersion(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("could not open OAS file: %w", err)
	}
	defer f.Close()

	var oas OpenAPIInfo
	if err := json.NewDecoder(f).Decode(&oas); err != nil {
		return "", fmt.Errorf("could not parse OAS: %w", err)
	}

	if oas.Info.Version == "" {
		return "", fmt.Errorf("version missing from OAS")
	}

	return oas.Info.Version, nil
}
