package redact

import "regexp"

var secretPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9_-]{8,}\b`), `[REDACTED]`},
	{regexp.MustCompile(`(?i)(authorization\s*[:=]\s*bearer\s+)[^\s\"']+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)[^\s,;\"']+`), `${1}[REDACTED]`},
}

func String(value string) string {
	redacted := value
	for _, entry := range secretPatterns {
		redacted = entry.pattern.ReplaceAllString(redacted, entry.replacement)
	}
	return redacted
}
