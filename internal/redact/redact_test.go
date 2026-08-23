package redact

import (
	"strings"
	"testing"
)

func TestStringRedactsSecrets(t *testing.T) {
	input := "Authorization: Bearer abc.def api_key=sk-secretvalue123"
	output := String(input)
	if strings.Contains(output, "abc.def") || strings.Contains(output, "sk-secretvalue123") {
		t.Fatalf("secret leaked: %s", output)
	}
}
