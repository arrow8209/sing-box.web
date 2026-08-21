package templates

import _ "embed"

//go:embed assets/default-profile.json
var defaultProfile []byte

//go:embed assets/default-rule-resources.json
var defaultResources []byte

//go:embed assets/default-bindings.json
var defaultBindings []byte
