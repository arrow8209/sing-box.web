package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory      uint32 = 65536
	argon2Iterations  uint32 = 3
	argon2Parallelism uint8  = 2
	argon2SaltLength         = 16
	argon2KeyLength          = 32
)

var ErrWeakPassword = errors.New("password must contain at least 6 Unicode code points")

type phcHash struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	key         []byte
}

func HashPassword(password []byte, random io.Reader) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := io.ReadFull(random, salt); err != nil {
		return "", err
	}
	key := argon2.IDKey(password, salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	defer clear(key)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(password []byte, encoded string) (bool, error) {
	parsed, err := parsePHC(encoded)
	if err != nil {
		return false, err
	}
	if parsed.memory != argon2Memory || parsed.iterations != argon2Iterations || parsed.parallelism != argon2Parallelism {
		return false, errors.New("unsupported Argon2id parameters")
	}
	calculated := argon2.IDKey(password, parsed.salt, parsed.iterations, parsed.memory, parsed.parallelism, uint32(len(parsed.key)))
	defer clear(calculated)
	return subtle.ConstantTimeCompare(calculated, parsed.key) == 1, nil
}

func parsePHC(encoded string) (phcHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return phcHash{}, errors.New("invalid Argon2id PHC string")
	}

	params := make(map[string]string, 3)
	for _, part := range strings.Split(parts[3], ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || key == "" || value == "" {
			return phcHash{}, errors.New("invalid Argon2id parameters")
		}
		if key != "m" && key != "t" && key != "p" {
			return phcHash{}, errors.New("invalid Argon2id parameters")
		}
		if _, exists := params[key]; exists {
			return phcHash{}, errors.New("invalid Argon2id parameters")
		}
		params[key] = value
	}
	if len(params) != 3 {
		return phcHash{}, errors.New("invalid Argon2id parameters")
	}
	memory, err := parseUint32(params["m"])
	if err != nil {
		return phcHash{}, err
	}
	iterations, err := parseUint32(params["t"])
	if err != nil {
		return phcHash{}, err
	}
	parallelism, err := parseUint8(params["p"])
	if err != nil {
		return phcHash{}, err
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != argon2SaltLength {
		return phcHash{}, errors.New("invalid Argon2id salt")
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(key) != argon2KeyLength {
		return phcHash{}, errors.New("invalid Argon2id key")
	}
	return phcHash{memory: memory, iterations: iterations, parallelism: parallelism, salt: salt, key: key}, nil
}

func parseUint32(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || parsed == 0 {
		return 0, errors.New("invalid Argon2id parameter")
	}
	return uint32(parsed), nil
}

func parseUint8(value string) (uint8, error) {
	parsed, err := strconv.ParseUint(value, 10, 8)
	if err != nil || parsed == 0 {
		return 0, errors.New("invalid Argon2id parameter")
	}
	return uint8(parsed), nil
}
