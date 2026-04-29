package apispec

import _ "embed"

//go:embed openapi.json
var openAPIJSON []byte

func OpenAPIJSON() []byte {
	return append([]byte(nil), openAPIJSON...)
}
