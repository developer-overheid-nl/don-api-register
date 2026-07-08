package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPStatusErrorErrorTrimsBody(t *testing.T) {
	err := &HTTPStatusError{
		StatusCode: 404,
		Body:       " not found\n",
	}

	assert.Equal(t, "kan OAS niet ophalen: status 404: not found", err.Error())
}
