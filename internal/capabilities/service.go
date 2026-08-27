package capabilities

// Response declares which API domains the embedded Web UI may use.
type Response struct {
	Configuration    bool `json:"configuration"`
	InboundResources bool `json:"inboundResources"`
	Subscriptions    bool `json:"subscriptions"`
	RuleResources    bool `json:"ruleResources"`
	Application      bool `json:"application"`
	CoreUpdate       bool `json:"coreUpdate"`
	Runtime          bool `json:"runtime"`
	Traffic          bool `json:"traffic"`
	Connections      bool `json:"connections"`
	ProxyGroups      bool `json:"proxyGroups"`
}

// PhaseOne contains only the domains implemented by the first delivery.
var PhaseOne = Response{
	Configuration:    true,
	InboundResources: true,
	Subscriptions:    true,
	RuleResources:    true,
	Application:      false,
	CoreUpdate:       true,
	Runtime:          false,
	Traffic:          false,
	Connections:      false,
	ProxyGroups:      false,
}
