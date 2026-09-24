package configgen

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"singbox-web/internal/configref"
)

// The accepted subset has the same meaning as a flagless JavaScript RegExp.
// Go-only inline flags are invalid JS and use the legacy literal fallback.
// Ambiguous escapes/extensions are rejected instead of silently changing meaning.
func referenceFilter(pattern, path string) (func(string) (bool, error), error) {
	if pattern == "" {
		return func(string) (bool, error) { return true, nil }, nil
	}
	unsupported := func() error {
		return configref.Error("reference_filter_unsupported", path, "过滤表达式不能等价处理，请改用普通文字或基础字符集合")
	}
	literal := func() (func(string) (bool, error), error) {
		return func(tag string) (bool, error) { return strings.Contains(tag, pattern), nil }, nil
	}
	var translated strings.Builder
	class := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' {
			if i+1 == len(pattern) {
				return literal()
			}
			i++
			escaped := pattern[i]
			if strings.ContainsRune("dDwWbBnrtfv\\.*+?^${}()|[]/-", rune(escaped)) {
				if class && strings.ContainsRune("bBdDwW", rune(escaped)) {
					return nil, unsupported()
				}
				if escaped == '/' {
					translated.WriteByte('/')
					continue
				}
				translated.WriteByte('\\')
				translated.WriteByte(escaped)
				continue
			}
			return nil, unsupported()
		}
		if !class && c == '(' && i+1 < len(pattern) && pattern[i+1] == '?' {
			if i+2 >= len(pattern) {
				return literal()
			}
			if pattern[i+2] == '=' || pattern[i+2] == '!' || pattern[i+2] == '<' {
				return nil, unsupported()
			}
			if pattern[i+2] != ':' {
				return literal()
			}
		}
		if c == '[' && class {
			return nil, unsupported()
		}
		if c == '[' && !class {
			class = true
		} else if c == ']' && class {
			class = false
		}
		if c == '.' && !class {
			translated.WriteString(`[^\n\r\x{2028}\x{2029}]`)
		} else {
			translated.WriteByte(c)
		}
	}
	// RE2 and ECMAScript disagree about empty/negated-empty character sets.
	if strings.Contains(pattern, "[]") || strings.Contains(pattern, "[^]") {
		return nil, unsupported()
	}
	compiled, err := regexp.Compile(translated.String())
	if err != nil {
		// Excessive repetition is valid JS but exceeds RE2's bound.
		if strings.Contains(err.Error(), "invalid repeat count") {
			return nil, unsupported()
		}
		return literal()
	}
	return func(tag string) (bool, error) {
		for _, r := range pattern + tag {
			if r > 0xffff || r == utf8.RuneError {
				return false, unsupported()
			}
		}
		return compiled.MatchString(tag), nil
	}, nil
}
