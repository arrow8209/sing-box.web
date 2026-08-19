package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
)

var ErrRevisionConflict = errors.New("draft revision conflict")

type RevisionConflict struct {
	Current uint64
}

func (conflict *RevisionConflict) Error() string {
	return fmt.Sprintf("%v: current revision is %d", ErrRevisionConflict, conflict.Current)
}

func (conflict *RevisionConflict) Unwrap() error { return ErrRevisionConflict }

var ErrDraftCorrupt = errors.New("draft JSON is corrupt")

const draftCorruptMessage = "草稿 JSON 损坏，需要管理员在服务器修复"

type DraftStatus struct {
	ReadOnly bool   `json:"readOnly"`
	Message  string `json:"message"`
}

func normalizeDraftContent(content DraftContent) (DraftContent, error) {
	configValue, err := decodeOneJSON(content.Config)
	if err != nil {
		return DraftContent{}, fmt.Errorf("invalid draft config: %s", jsonErrorMessage(err, 0))
	}
	configObject, ok := configValue.(map[string]any)
	if !ok {
		return DraftContent{}, fmt.Errorf("draft config must be one top-level JSON object")
	}

	normalized := DraftContent{
		Config:        append(json.RawMessage(nil), content.Config...),
		EditorBuffers: make(map[string]EditorBuffer, len(content.EditorBuffers)),
	}
	for key, supplied := range content.EditorBuffers {
		segments, err := safeBufferPath(key)
		if err != nil {
			return DraftContent{}, err
		}
		buffer := EditorBuffer{Text: supplied.Text}
		parsed, parseErr := decodeOneJSON([]byte(supplied.Text))
		if parseErr != nil {
			buffer.Error = jsonErrorMessage(parseErr, 0)
			normalized.EditorBuffers[key] = buffer
			continue
		}
		buffer.Valid = true
		canonical, exists := configPathValue(configObject, segments)
		if len(segments) == 1 && exists {
			canonical = map[string]any{segments[0]: canonical}
		}
		if exists && semanticJSONEqual(parsed, canonical) {
			buffer.Applied = true
		} else {
			buffer.Error = "缓冲与配置对象不同步"
		}
		normalized.EditorBuffers[key] = buffer
	}
	return normalized, nil
}

func IsPublishable(draft Draft) bool {
	value, err := decodeOneJSON(draft.Content.Config)
	if err != nil {
		return false
	}
	if _, ok := value.(map[string]any); !ok {
		return false
	}
	for _, buffer := range draft.Content.EditorBuffers {
		if !buffer.Valid || !buffer.Applied {
			return false
		}
	}
	return true
}

type multipleJSONValuesError struct{ offset int64 }

func (err *multipleJSONValuesError) Error() string { return "JSON contains trailing value" }

type jsonDecodeError struct {
	err    error
	offset int64
}

func (err *jsonDecodeError) Error() string { return err.err.Error() }
func (err *jsonDecodeError) Unwrap() error { return err.err }

func decodeOneJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		offset := decoder.InputOffset()
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			offset = syntax.Offset
		} else if errors.Is(err, io.ErrUnexpectedEOF) {
			offset = int64(len(raw) + 1)
		}
		return nil, &jsonDecodeError{err: err, offset: offset}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, &multipleJSONValuesError{offset: decoder.InputOffset()}
		}
		return nil, err
	}
	return value, nil
}

func jsonErrorMessage(err error, fallbackOffset int64) string {
	var decodeErr *jsonDecodeError
	if errors.As(err, &decodeErr) {
		fallbackOffset = decodeErr.offset
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Sprintf("JSON 语法错误（偏移 %d）", syntax.Offset)
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("JSON 类型错误（偏移 %d）", typeErr.Offset)
	}
	var multiple *multipleJSONValuesError
	if errors.As(err, &multiple) {
		return fmt.Sprintf("JSON 包含多个值（偏移 %d）", multiple.offset)
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Sprintf("JSON 意外结束（偏移 %d）", fallbackOffset)
	}
	return fmt.Sprintf("JSON 解析错误（偏移 %d）", fallbackOffset)
}

func safeBufferPath(key string) ([]string, error) {
	segments := strings.Split(key, "/")
	for _, segment := range segments {
		switch segment {
		case "", ".", "..", "__proto__", "prototype", "constructor":
			return nil, fmt.Errorf("unsafe editor buffer path %q", key)
		}
	}
	return segments, nil
}

func configPathValue(root map[string]any, segments []string) (any, bool) {
	var current any = root
	for _, segment := range segments {
		switch typed := current.(type) {
		case map[string]any:
			var exists bool
			current, exists = typed[segment]
			if !exists {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) || strconv.Itoa(index) != segment {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func semanticJSONEqual(left, right any) bool {
	switch leftValue := left.(type) {
	case nil:
		return right == nil
	case bool:
		rightValue, ok := right.(bool)
		return ok && leftValue == rightValue
	case string:
		rightValue, ok := right.(string)
		return ok && leftValue == rightValue
	case json.Number:
		rightValue, ok := right.(json.Number)
		if !ok {
			return false
		}
		leftNumber, leftOK := exactJSONNumber(leftValue.String())
		rightNumber, rightOK := exactJSONNumber(rightValue.String())
		return leftOK && rightOK && leftNumber.equal(rightNumber)
	case []any:
		rightValue, ok := right.([]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for index := range leftValue {
			if !semanticJSONEqual(leftValue[index], rightValue[index]) {
				return false
			}
		}
		return true
	case map[string]any:
		rightValue, ok := right.(map[string]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for key, value := range leftValue {
			rightItem, exists := rightValue[key]
			if !exists || !semanticJSONEqual(value, rightItem) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

type normalizedJSONNumber struct {
	negative bool
	digits   string
	power    *big.Int
}

func (number normalizedJSONNumber) equal(other normalizedJSONNumber) bool {
	if number.digits == "" || other.digits == "" {
		return number.digits == "" && other.digits == ""
	}
	return number.negative == other.negative && number.digits == other.digits && number.power.Cmp(other.power) == 0
}

func exactJSONNumber(raw string) (normalizedJSONNumber, bool) {
	lower := strings.ToLower(raw)
	parts := strings.SplitN(lower, "e", 2)
	mantissa := parts[0]
	exponent := new(big.Int)
	if len(parts) == 2 {
		if _, ok := exponent.SetString(parts[1], 10); !ok {
			return normalizedJSONNumber{}, false
		}
	}
	negative := strings.HasPrefix(mantissa, "-")
	mantissa = strings.TrimPrefix(mantissa, "-")
	fractionDigits := 0
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		fractionDigits = len(mantissa) - dot - 1
		mantissa = mantissa[:dot] + mantissa[dot+1:]
	}
	digits := strings.TrimLeft(mantissa, "0")
	if digits == "" {
		return normalizedJSONNumber{}, true
	}
	if strings.IndexFunc(digits, func(value rune) bool { return value < '0' || value > '9' }) >= 0 {
		return normalizedJSONNumber{}, false
	}
	trailingZeros := len(digits) - len(strings.TrimRight(digits, "0"))
	if trailingZeros > 0 {
		digits = digits[:len(digits)-trailingZeros]
	}
	power := new(big.Int).Set(exponent)
	power.Sub(power, big.NewInt(int64(fractionDigits)))
	power.Add(power, big.NewInt(int64(trailingZeros)))
	return normalizedJSONNumber{negative: negative, digits: digits, power: power}, true
}
