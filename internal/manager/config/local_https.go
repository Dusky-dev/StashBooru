package config

import "strings"

const (
	LocalHTTPSEnabled  = "local_https.enabled"
	LocalHTTPSPort     = "local_https.port"
	LocalHTTPSHosts    = "local_https.hosts"
	LocalHTTPSCertPath = "local_https.cert_path"
	LocalHTTPSKeyPath  = "local_https.key_path"

	localHTTPSEnabledDefault = true
	localHTTPSPortDefault    = 9443
)

// GetLocalHTTPSEnabled reports whether the secondary local HTTPS listener is enabled.
// It defaults to true so a normal HTTP installation also has an encrypted local endpoint.
func (i *Config) GetLocalHTTPSEnabled() bool {
	return i.getBoolDefault(LocalHTTPSEnabled, localHTTPSEnabledDefault)
}

// GetLocalHTTPSPort returns the port used by the secondary local HTTPS listener.
func (i *Config) GetLocalHTTPSPort() int {
	ret := i.getInt(LocalHTTPSPort)
	if ret <= 0 {
		ret = localHTTPSPortDefault
	}

	return ret
}

// GetLocalHTTPSHosts returns additional DNS names or IP addresses that should be
// included in the generated local certificate. YAML lists and comma/semicolon/
// whitespace-separated strings are both accepted so environment overrides are easy.
func (i *Config) GetLocalHTTPSHosts() []string {
	values := i.getStringSlice(LocalHTTPSHosts)
	if len(values) == 0 {
		if raw := i.getString(LocalHTTPSHosts); raw != "" {
			values = []string{raw}
		}
	}

	var ret []string
	for _, value := range values {
		ret = append(ret, strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
		})...)
	}
	return ret
}

func (i *Config) GetLocalHTTPSCertPath() string {
	return i.getString(LocalHTTPSCertPath)
}

func (i *Config) GetLocalHTTPSKeyPath() string {
	return i.getString(LocalHTTPSKeyPath)
}
