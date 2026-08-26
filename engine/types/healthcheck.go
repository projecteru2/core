package types

// HealthCheck is the probe a workload record hands to eru-agent.
type HealthCheck struct {
	TCPPorts []string `json:"tcp_ports,omitempty"`
	HTTPPort string   `json:"http_port,omitempty"`
	HTTPURL  string   `json:"http_url,omitempty"`
	HTTPCode int      `json:"http_code,omitempty"`
}
