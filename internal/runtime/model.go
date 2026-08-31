package runtime

// ConnectionsSnapshot is the stable subset of sing-box's Clash API
// connection snapshot exposed to the Web UI.
type ConnectionsSnapshot struct {
	Memory        int64        `json:"memory"`
	UploadTotal   int64        `json:"uploadTotal"`
	DownloadTotal int64        `json:"downloadTotal"`
	Connections   []Connection `json:"connections"`
}

// Connection keeps metadata open-ended because sing-box adds fields as
// protocol sniffing and platform process discovery evolve.
type Connection struct {
	ID          string         `json:"id"`
	Metadata    map[string]any `json:"metadata"`
	Upload      int64          `json:"upload"`
	Download    int64          `json:"download"`
	Start       string         `json:"start"`
	Chains      []string       `json:"chains"`
	Rule        string         `json:"rule"`
	RulePayload string         `json:"rulePayload"`
}
