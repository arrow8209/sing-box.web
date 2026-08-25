// Package configgen turns a saved profile draft into the managed sing-box
// configuration directory.  It deliberately has no filesystem or network
// dependencies while generating an Output, so a caller can preview exactly
// what an apply operation will write.
package configgen

import "encoding/json"

// Module is one supported sing-box top-level section and its fixed output
// filename. Keeping this table closed prevents a draft from choosing paths in
// the configuration directory.
type Module struct {
	Key      string
	FileName string
}

var modules = []Module{
	{"log", "10-log.json"},
	{"dns", "20-dns.json"},
	{"ntp", "30-ntp.json"},
	{"certificate", "40-certificate.json"},
	{"certificate_providers", "41-certificate-providers.json"},
	{"http_clients", "42-http-clients.json"},
	{"network_namespaces", "43-network-namespaces.json"},
	{"inbounds", "60-inbounds.json"},
	{"outbounds", "70-outbounds.json"},
	{"route", "80-route.json"},
	{"services", "90-services.json"},
	{"experimental", "99-experimental.json"},
}

// File is one generated, self-contained configuration module.
type File struct {
	Name    string          `json:"name"`
	Content json.RawMessage `json:"content"`
}

// Output is a deterministic generated configuration and the rule digests
// captured while the corresponding resources were locked.
type Output struct {
	Files           []File            `json:"files"`
	Digest          string            `json:"digest"`
	ResourceDigests map[string]string `json:"resourceDigests"`
}

func knownFileName(name string) bool {
	for _, module := range modules {
		if module.FileName == name {
			return true
		}
	}
	return false
}
