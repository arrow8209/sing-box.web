package assets

import _ "embed"

//go:embed config.default.json
var DefaultConfig []byte

//go:embed sing-box.schema.json
var SingBoxSchema []byte

//go:embed schema.version.json
var SchemaVersion []byte
