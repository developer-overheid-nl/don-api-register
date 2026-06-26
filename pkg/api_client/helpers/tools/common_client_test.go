package tools

import (
	"strings"
	"testing"
)

func TestToolsErrorMessageUsesProblemDetail(t *testing.T) {
	message := toolsErrorMessage(
		"422 Unprocessable Entity",
		[]byte(`{"status":422,"title":"Unprocessable Entity","detail":"De OpenAPI specificatie bevat circulaire verwijzingen."}`),
	)

	if message != "De OpenAPI specificatie bevat circulaire verwijzingen." {
		t.Fatalf("expected problem detail, got %q", message)
	}
}

func TestToolsErrorMessageFallsBackToProblemTitle(t *testing.T) {
	message := toolsErrorMessage(
		"422 Unprocessable Entity",
		[]byte(`{"status":422,"title":"De OpenAPI specificatie bevat circulaire verwijzingen."}`),
	)

	if message != "De OpenAPI specificatie bevat circulaire verwijzingen." {
		t.Fatalf("expected problem title, got %q", message)
	}
}

func TestReadResponseBodyRejectsOversizedResponse(t *testing.T) {
	_, err := readResponseBody(strings.NewReader("abcdef"), 5)
	if err == nil {
		t.Fatal("expected oversized response error")
	}
}

func TestReadResponseBodyAllowsResponseAtLimit(t *testing.T) {
	data, err := readResponseBody(strings.NewReader("abcde"), 5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(data) != "abcde" {
		t.Fatalf("expected response body abcde, got %q", string(data))
	}
}
