// Package updater obtains a verified sing-box program but never installs it.
package updater

import "errors"

const (
	latestReleaseURL = "https://api.github.com/repos/SagerNet/sing-box/releases/latest"
	maxArchiveBytes  = int64(256 << 20)
	maxChecksumBytes = int64(4 << 20)
)

var (
	ErrUnsupportedArchitecture = errors.New("unsupported_architecture")
	ErrUntrustedURL            = errors.New("核心下载地址不受信任")
	ErrTooManyRedirects        = errors.New("核心下载重定向次数超过限制")
	ErrResponseTooLarge        = errors.New("核心下载响应超过限制")
	ErrChecksumNotFound        = errors.New("未找到对应核心压缩包摘要")
	ErrChecksumMismatch        = errors.New("核心压缩包摘要校验失败")
	ErrUnsafeArchive           = errors.New("核心压缩包不安全")
)

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// Release is the sole stable release and its verified archive accepted by this
// package. It is intentionally not a general GitHub release representation.
type Release struct {
	Version string
	Asset   Asset
	// Checksums supports releases published before GitHub exposed immutable
	// asset digests in the release API. New releases use ArchiveSHA256.
	Checksums     Asset
	ArchiveSHA256 string
	Arch          string
}

type PreparedBinary struct {
	Path          string
	Version       string
	ArchiveDigest string
	BinaryDigest  string
}

// DownloadProgress contains observable archive transfer facts. TotalBytes is
// zero when neither the release metadata nor the response declares a size.
type DownloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	BytesPerSecond  int64
}
