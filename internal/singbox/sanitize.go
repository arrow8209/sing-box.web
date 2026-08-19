package singbox

import "regexp"

// sensitiveAssignment matches exact, case-insensitive sensitive keys in
// JSON-like and key=value output. The prefix is kept so neighboring fields
// remain visible, while the value is replaced as one unit.
var sensitiveAssignment = regexp.MustCompile(`(?i)(^|[\s,{;])((?:"|')?(?:access_token|refresh_token|authorization|client_secret|private_key|pre_shared_key|password|passwd|secret|token|api_key|uuid)(?:"|')?\s*(?::|=)\s*)("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[Bb]earer[ \t]+[^\s,}\];]+|[^\s,}\];]+)`)

// Sanitize removes values assigned to the supported sensitive keys. Replacing
// filesystem roots is intentionally left to the HTTP caller, which knows the
// configured roots to redact.
func Sanitize(text string) string {
	return sensitiveAssignment.ReplaceAllString(text, "${1}${2}***REDACTED***")
}
