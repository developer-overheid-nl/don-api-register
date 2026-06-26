package tools

import "testing"

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
