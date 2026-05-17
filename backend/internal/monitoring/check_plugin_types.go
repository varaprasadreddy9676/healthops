package monitoring

// Config blocks for pluggable check types.
//
// These structs are defined centrally so they can be referenced by
// CheckConfig in types.go without cyclic imports. The actual executor
// implementations live in their own files (check_ssl.go, check_dns.go, etc.)
// and register themselves with the CheckExecutor registry via init().

// SSLCheckConfig configures TLS certificate expiry / chain checks.
type SSLCheckConfig struct {
	// Host to connect to (e.g. "example.com"). If empty, falls back to
	// CheckConfig.Host or the host portion of CheckConfig.Target.
	Host string `json:"host,omitempty" bson:"host,omitempty"`
	// Port defaults to 443.
	Port int `json:"port,omitempty" bson:"port,omitempty"`
	// ServerName overrides the SNI value sent in the TLS handshake. Defaults
	// to Host. Useful when the cert subject differs from the connect host.
	ServerName string `json:"serverName,omitempty" bson:"serverName,omitempty"`
	// WarningDays triggers a warning when leaf cert expires within this many
	// days. Defaults to 30.
	WarningDays int `json:"warningDays,omitempty" bson:"warningDays,omitempty"`
	// CriticalDays triggers a critical when leaf cert expires within this many
	// days. Defaults to 7. Must be <= WarningDays.
	CriticalDays int `json:"criticalDays,omitempty" bson:"criticalDays,omitempty"`
	// InsecureSkipVerify disables full chain validation (still reports expiry).
	// Use only for self-signed certs you trust by other means.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty" bson:"insecureSkipVerify,omitempty"`
}

// DNSCheckConfig configures DNS record monitoring.
type DNSCheckConfig struct {
	// Name is the DNS name to query (e.g. "example.com").
	Name string `json:"name" bson:"name"`
	// RecordType is one of: A, AAAA, CNAME, TXT, MX, NS. Defaults to "A".
	RecordType string `json:"recordType,omitempty" bson:"recordType,omitempty"`
	// Resolver is an optional DNS server "host:port". Empty uses system resolver.
	Resolver string `json:"resolver,omitempty" bson:"resolver,omitempty"`
	// Expected is the list of values the query MUST return (in any order).
	// For TXT, each entry is matched as a substring. Empty means "any answer".
	Expected []string `json:"expected,omitempty" bson:"expected,omitempty"`
	// MustNotContain — query must NOT return any of these values.
	MustNotContain []string `json:"mustNotContain,omitempty" bson:"mustNotContain,omitempty"`
}

// PingCheckConfig configures ICMP/UDP ping reachability checks.
type PingCheckConfig struct {
	// Host or IP to ping. Falls back to CheckConfig.Host / CheckConfig.Target.
	Host string `json:"host,omitempty" bson:"host,omitempty"`
	// Count is the number of probes per run. Defaults to 3.
	Count int `json:"count,omitempty" bson:"count,omitempty"`
	// MaxPacketLossPercent triggers critical above this loss percentage.
	// Defaults to 50.
	MaxPacketLossPercent int `json:"maxPacketLossPercent,omitempty" bson:"maxPacketLossPercent,omitempty"`
	// MaxAvgLatencyMs triggers warning above this average latency. 0 = disabled.
	MaxAvgLatencyMs int `json:"maxAvgLatencyMs,omitempty" bson:"maxAvgLatencyMs,omitempty"`
}

// DomainCheckConfig configures WHOIS/RDAP domain expiry monitoring.
type DomainCheckConfig struct {
	// Domain to query (e.g. "example.com"). Required.
	Domain string `json:"domain" bson:"domain"`
	// WarningDays triggers a warning when domain expires within this many days.
	// Defaults to 30.
	WarningDays int `json:"warningDays,omitempty" bson:"warningDays,omitempty"`
	// CriticalDays triggers a critical within this many days. Defaults to 7.
	CriticalDays int `json:"criticalDays,omitempty" bson:"criticalDays,omitempty"`
	// RDAPEndpoint overrides RDAP base URL. Empty = use IANA bootstrap.
	RDAPEndpoint string `json:"rdapEndpoint,omitempty" bson:"rdapEndpoint,omitempty"`
}

// HeartbeatCheckConfig configures push-based heartbeat / cron monitoring.
//
// A unique Token is assigned at create time. Clients POST or GET
// /api/v1/heartbeats/{token} to record a ping. The check passes if the most
// recent ping is within (ExpectedIntervalSeconds + GraceSeconds) of "now".
type HeartbeatCheckConfig struct {
	// Token is the unguessable token in the ping URL. Auto-generated when empty.
	Token string `json:"token,omitempty" bson:"token,omitempty"`
	// ExpectedIntervalSeconds is how often pings are expected. Required.
	ExpectedIntervalSeconds int `json:"expectedIntervalSeconds" bson:"expectedIntervalSeconds"`
	// GraceSeconds is the additional wait before marking late. Defaults to 60.
	GraceSeconds int `json:"graceSeconds,omitempty" bson:"graceSeconds,omitempty"`
}
