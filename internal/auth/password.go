package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltBytes   uint32
	KeyBytes    uint32
}

var passwordParams = Params{
	MemoryKiB:   64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltBytes:   16,
	KeyBytes:    32,
}

func Hash(password []byte, random io.Reader) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	if random == nil {
		return "", fmt.Errorf("随机源不能为空")
	}
	salt := make([]byte, passwordParams.SaltBytes)
	if _, err := io.ReadFull(random, salt); err != nil {
		return "", err
	}
	key := argon2.IDKey(password, salt, passwordParams.Iterations, passwordParams.MemoryKiB, passwordParams.Parallelism, passwordParams.KeyBytes)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, passwordParams.MemoryKiB, passwordParams.Iterations, passwordParams.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func Verify(password []byte, encoded string) (bool, error) {
	params, salt, expected, err := parseHash(encoded)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey(password, salt, params.Iterations, params.MemoryKiB, params.Parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func validatePassword(password []byte) error {
	if len(password) > 1024 {
		return fmt.Errorf("密码长度不能超过 1024 字节")
	}
	if !utf8.Valid(password) {
		return fmt.Errorf("密码必须是有效的 UTF-8 文本")
	}
	if utf8.RuneCount(password) < 6 {
		return fmt.Errorf("密码至少需要 6 个字符")
	}
	return nil
}

func parseHash(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return Params{}, nil, nil, fmt.Errorf("无效的密码哈希格式")
	}
	parameterParts := strings.Split(parts[3], ",")
	if len(parameterParts) != 3 {
		return Params{}, nil, nil, fmt.Errorf("无效的密码哈希参数")
	}
	memory, memoryErr := parseParameter(parameterParts[0], "m=")
	iterations, iterationsErr := parseParameter(parameterParts[1], "t=")
	parallelism, parallelismErr := parseParameter(parameterParts[2], "p=")
	if memoryErr != nil || iterationsErr != nil || parallelismErr != nil {
		return Params{}, nil, nil, fmt.Errorf("无效的密码哈希参数")
	}
	if memory == 0 || memory > uint64(passwordParams.MemoryKiB) || iterations == 0 || iterations > uint64(passwordParams.Iterations) || parallelism == 0 || parallelism > uint64(passwordParams.Parallelism) {
		return Params{}, nil, nil, fmt.Errorf("密码哈希参数超出允许范围")
	}
	if memory > uint64(^uint32(0)) || iterations > uint64(^uint32(0)) || parallelism > uint64(^uint8(0)) {
		return Params{}, nil, nil, fmt.Errorf("无效的密码哈希参数")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != int(passwordParams.SaltBytes) {
		return Params{}, nil, nil, fmt.Errorf("无效的密码哈希盐值")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != int(passwordParams.KeyBytes) {
		return Params{}, nil, nil, fmt.Errorf("无效的密码哈希密钥")
	}
	return Params{MemoryKiB: uint32(memory), Iterations: uint32(iterations), Parallelism: uint8(parallelism), SaltBytes: uint32(len(salt)), KeyBytes: uint32(len(key))}, salt, key, nil
}

func parseParameter(value, prefix string) (uint64, error) {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return 0, fmt.Errorf("缺少参数")
	}
	return strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, 64)
}
