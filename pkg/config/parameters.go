// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	contour_api_v1alpha1 "github.com/projectcontour/contour/apis/projectcontour/v1alpha1"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ServerType is the name of a xDS server implementation.
type ServerType string

const ContourServerType ServerType = "contour"
const EnvoyServerType ServerType = "envoy"

// Validate the xDS server type.
func (s ServerType) Validate() error {
	switch s {
	case ContourServerType, EnvoyServerType:
		return nil
	default:
		return fmt.Errorf("invalid xDS server type %q", s)
	}
}

// Validate the GatewayConfig.
func (g *GatewayParameters) Validate() error {
	if g == nil {
		return nil
	}

	if len(g.ControllerName) == 0 && g.GatewayRef == nil {
		return fmt.Errorf("invalid Gateway parameters specified: exactly one of controller name or gateway ref must be provided")
	}

	if len(g.ControllerName) > 0 && g.GatewayRef != nil {
		return fmt.Errorf("invalid Gateway parameters specified: exactly one of controller name or gateway ref must be provided")
	}

	return nil
}

// ResourceVersion is a version of an xDS server.
type ResourceVersion string

const XDSv3 ResourceVersion = "v3"

// Validate the xDS server versions.
func (s ResourceVersion) Validate() error {
	switch s {
	case XDSv3:
		return nil
	default:
		return fmt.Errorf("invalid xDS version %q", s)
	}
}

// ClusterDNSFamilyType is the Ip family to use for resolving DNS
// names in an Envoy cluster configuration.
type ClusterDNSFamilyType string

func (c ClusterDNSFamilyType) Validate() error {
	switch c {
	case AutoClusterDNSFamily, IPv4ClusterDNSFamily, IPv6ClusterDNSFamily:
		return nil
	default:
		return fmt.Errorf("invalid cluster DNS lookup family %q", c)
	}
}

const AutoClusterDNSFamily ClusterDNSFamilyType = "auto"
const IPv4ClusterDNSFamily ClusterDNSFamilyType = "v4"
const IPv6ClusterDNSFamily ClusterDNSFamilyType = "v6"

// AccessLogType is the name of a supported access logging mechanism.
type AccessLogType string

func (a AccessLogType) Validate() error {
	return contour_api_v1alpha1.AccessLogType(a).Validate()
}

const EnvoyAccessLog AccessLogType = "envoy"
const JSONAccessLog AccessLogType = "json"

type AccessLogFields []string

func (a AccessLogFields) Validate() error {
	return contour_api_v1alpha1.AccessLogJSONFields(a).Validate()
}

func (a AccessLogFields) AsFieldMap() map[string]string {
	return contour_api_v1alpha1.AccessLogJSONFields(a).AsFieldMap()
}

// AccessLogFormatterExtensions returns a list of formatter extension names required by the access log format.
func (p Parameters) AccessLogFormatterExtensions() []string {
	el := &contour_api_v1alpha1.EnvoyLogging{
		AccessLogFormat:       contour_api_v1alpha1.AccessLogType(p.AccessLogFormat),
		AccessLogFormatString: p.AccessLogFormatString,
		AccessLogJSONFields:   contour_api_v1alpha1.AccessLogJSONFields(p.AccessLogFields),
		AccessLogLevel:        contour_api_v1alpha1.AccessLogLevel(p.AccessLogLevel),
	}
	return el.AccessLogFormatterExtensions()
}

// HTTPVersionType is the name of a supported HTTP version.
type HTTPVersionType string

func (h HTTPVersionType) Validate() error {
	switch h {
	case HTTPVersion1, HTTPVersion2:
		return nil
	default:
		return fmt.Errorf("invalid HTTP version %q", h)
	}
}

const HTTPVersion1 HTTPVersionType = "http/1.1"
const HTTPVersion2 HTTPVersionType = "http/2"

// NamespacedName defines the namespace/name of the Kubernetes resource referred from the configuration file.
// Used for Contour configuration YAML file parsing, otherwise we could use K8s types.NamespacedName.
type NamespacedName struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// Validate that both name fields are present, or neither are.
func (n NamespacedName) Validate() error {
	if len(strings.TrimSpace(n.Name)) == 0 && len(strings.TrimSpace(n.Namespace)) == 0 {
		return nil
	}

	if len(strings.TrimSpace(n.Namespace)) == 0 {
		return errors.New("namespace must be defined")
	}

	if len(strings.TrimSpace(n.Name)) == 0 {
		return errors.New("name must be defined")
	}

	return nil
}

// TLSParameters holds configuration file TLS configuration details.
type TLSParameters struct {
	MinimumProtocolVersion string `yaml:"minimum-protocol-version"`

	// FallbackCertificate defines the namespace/name of the Kubernetes secret to
	// use as fallback when a non-SNI request is received.
	FallbackCertificate NamespacedName `yaml:"fallback-certificate,omitempty"`

	// ClientCertificate defines the namespace/name of the Kubernetes
	// secret containing the client certificate and private key
	// to be used when establishing TLS connection to upstream
	// cluster.
	ClientCertificate NamespacedName `yaml:"envoy-client-certificate,omitempty"`

	// CipherSuites defines the TLS ciphers to be supported by Envoy TLS
	// listeners when negotiating TLS 1.2. Ciphers are validated against the
	// set that Envoy supports by default. This parameter should only be used
	// by advanced users. Note that these will be ignored when TLS 1.3 is in
	// use.
	CipherSuites TLSCiphers `yaml:"cipher-suites,omitempty"`
}

// Validate TLS fallback certificate, client certificate, and cipher suites
func (t TLSParameters) Validate() error {
	// Check TLS secret names.
	if err := t.FallbackCertificate.Validate(); err != nil {
		return fmt.Errorf("invalid TLS fallback certificate: %w", err)
	}

	if err := t.ClientCertificate.Validate(); err != nil {
		return fmt.Errorf("invalid TLS client certificate: %w", err)
	}

	if err := t.CipherSuites.Validate(); err != nil {
		return fmt.Errorf("invalid TLS cipher suites: %w", err)
	}

	return nil
}

// ServerParameters holds the configuration for the Contour xDS server.
type ServerParameters struct {
	// Defines the XDSServer to use for `contour serve`.
	// Defaults to "contour"
	XDSServerType ServerType `yaml:"xds-server-type,omitempty"`
}

// GatewayParameters holds the configuration for Gateway API controllers.
type GatewayParameters struct {
	// ControllerName is used to determine whether Contour should reconcile a
	// GatewayClass. The string takes the form of "projectcontour.io/<namespace>/contour".
	// If unset, the gatewayclass controller will not be started.
	// Exactly one of ControllerName or GatewayRef must be set.
	ControllerName string `yaml:"controllerName,omitempty"`

	// GatewayRef defines a specific Gateway that this Contour
	// instance corresponds to. If set, Contour will reconcile
	// only this gateway, and will not reconcile any gateway
	// classes.
	// Exactly one of ControllerName or GatewayRef must be set.
	GatewayRef *NamespacedName `yaml:"gatewayRef,omitempty"`
}

// TimeoutParameters holds various configurable proxy timeout values.
type TimeoutParameters struct {
	// RequestTimeout sets the client request timeout globally for Contour. Note that
	// this is a timeout for the entire request, not an idle timeout. Omit or set to
	// "infinity" to disable the timeout entirely.
	//
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto#envoy-v3-api-field-extensions-filters-network-http-connection-manager-v3-httpconnectionmanager-request-timeout
	// for more information.
	RequestTimeout string `yaml:"request-timeout,omitempty"`

	// ConnectionIdleTimeout defines how long the proxy should wait while there are
	// no active requests (for HTTP/1.1) or streams (for HTTP/2) before terminating
	// an HTTP connection. Set to "infinity" to disable the timeout entirely.
	//
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/core/v3/protocol.proto#envoy-v3-api-field-config-core-v3-httpprotocoloptions-idle-timeout
	// for more information.
	ConnectionIdleTimeout string `yaml:"connection-idle-timeout,omitempty"`

	// StreamIdleTimeout defines how long the proxy should wait while there is no
	// request activity (for HTTP/1.1) or stream activity (for HTTP/2) before
	// terminating the HTTP request or stream. Set to "infinity" to disable the
	// timeout entirely.
	//
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto#envoy-v3-api-field-extensions-filters-network-http-connection-manager-v3-httpconnectionmanager-stream-idle-timeout
	// for more information.
	StreamIdleTimeout string `yaml:"stream-idle-timeout,omitempty"`

	// MaxConnectionDuration defines the maximum period of time after an HTTP connection
	// has been established from the client to the proxy before it is closed by the proxy,
	// regardless of whether there has been activity or not. Omit or set to "infinity" for
	// no max duration.
	//
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/core/v3/protocol.proto#envoy-v3-api-field-config-core-v3-httpprotocoloptions-max-connection-duration
	// for more information.
	MaxConnectionDuration string `yaml:"max-connection-duration,omitempty"`

	// DelayedCloseTimeout defines how long envoy will wait, once connection
	// close processing has been initiated, for the downstream peer to close
	// the connection before Envoy closes the socket associated with the connection.
	//
	// Setting this timeout to 'infinity' will disable it, equivalent to setting it to '0'
	// in Envoy. Leaving it unset will result in the Envoy default value being used.
	//
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto#envoy-v3-api-field-extensions-filters-network-http-connection-manager-v3-httpconnectionmanager-delayed-close-timeout
	// for more information.
	DelayedCloseTimeout string `yaml:"delayed-close-timeout,omitempty"`

	// ConnectionShutdownGracePeriod defines how long the proxy will wait between sending an
	// initial GOAWAY frame and a second, final GOAWAY frame when terminating an HTTP/2 connection.
	// During this grace period, the proxy will continue to respond to new streams. After the final
	// GOAWAY frame has been sent, the proxy will refuse new streams.
	//
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto#envoy-v3-api-field-extensions-filters-network-http-connection-manager-v3-httpconnectionmanager-drain-timeout
	// for more information.
	ConnectionShutdownGracePeriod string `yaml:"connection-shutdown-grace-period,omitempty"`

	// ConnectTimeout defines how long the proxy should wait when establishing connection to upstream service.
	// If not set, a default value of 2 seconds will be used.
	//
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/cluster/v3/cluster.proto#envoy-v3-api-field-config-cluster-v3-cluster-connect-timeout
	// for more information.
	// +optional
	ConnectTimeout string `yaml:"connect-timeout,omitempty"`
}

// Validate the timeout parameters.
func (t TimeoutParameters) Validate() error {
	// We can't use `timeout.Parse` for validation here because
	// that would make an exported package depend on an internal
	// package.
	v := func(str string) error {
		switch str {
		case "", "infinity", "infinite":
			return nil
		default:
			_, err := time.ParseDuration(str)
			return err
		}
	}

	if err := v(t.RequestTimeout); err != nil {
		return fmt.Errorf("invalid request timeout %q: %w", t.RequestTimeout, err)
	}

	if err := v(t.ConnectionIdleTimeout); err != nil {
		return fmt.Errorf("connection idle timeout %q: %w", t.ConnectionIdleTimeout, err)
	}

	if err := v(t.StreamIdleTimeout); err != nil {
		return fmt.Errorf("stream idle timeout %q: %w", t.StreamIdleTimeout, err)
	}

	if err := v(t.MaxConnectionDuration); err != nil {
		return fmt.Errorf("max connection duration %q: %w", t.MaxConnectionDuration, err)
	}

	if err := v(t.DelayedCloseTimeout); err != nil {
		return fmt.Errorf("delayed close timeout %q: %w", t.DelayedCloseTimeout, err)
	}

	if err := v(t.ConnectionShutdownGracePeriod); err != nil {
		return fmt.Errorf("connection shutdown grace period %q: %w", t.ConnectionShutdownGracePeriod, err)
	}

	// ConnectTimeout is normally implicitly set to 2s in Defaults().
	// ConnectTimeout cannot be "infinite" so use time.ParseDuration() directly instead of v().
	if t.ConnectTimeout != "" {
		if _, err := time.ParseDuration(t.ConnectTimeout); err != nil {
			return fmt.Errorf("connect timeout %q: %w", t.ConnectTimeout, err)
		}
	}

	return nil
}

type HeadersPolicy struct {
	Set    map[string]string `yaml:"set,omitempty"`
	Remove []string          `yaml:"remove,omitempty"`
}

func (h HeadersPolicy) Validate() error {
	for key := range h.Set {
		if msgs := validation.IsHTTPHeaderName(key); len(msgs) != 0 {
			return fmt.Errorf("invalid header name %q: %v", key, msgs)
		}
	}
	for _, val := range h.Remove {
		if msgs := validation.IsHTTPHeaderName(val); len(msgs) != 0 {
			return fmt.Errorf("invalid header name %q: %v", val, msgs)
		}
	}
	return nil
}

// PolicyParameters holds default policy used if not explicitly set by the user
type PolicyParameters struct {
	// RequestHeadersPolicy defines the request headers set/removed on all routes
	RequestHeadersPolicy HeadersPolicy `yaml:"request-headers,omitempty"`

	// ResponseHeadersPolicy defines the response headers set/removed on all routes
	ResponseHeadersPolicy HeadersPolicy `yaml:"response-headers,omitempty"`

	// ApplyToIngress determines if the Policies will apply to ingress objects
	ApplyToIngress bool `yaml:"applyToIngress,omitempty"`
}

// Validate the header parameters.
func (h PolicyParameters) Validate() error {
	if err := h.RequestHeadersPolicy.Validate(); err != nil {
		return err
	}
	return h.ResponseHeadersPolicy.Validate()
}

// ClusterParameters holds various configurable cluster values.
type ClusterParameters struct {
	// DNSLookupFamily defines how external names are looked up
	// When configured as V4, the DNS resolver will only perform a lookup
	// for addresses in the IPv4 family. If V6 is configured, the DNS resolver
	// will only perform a lookup for addresses in the IPv6 family.
	// If AUTO is configured, the DNS resolver will first perform a lookup
	// for addresses in the IPv6 family and fallback to a lookup for addresses
	// in the IPv4 family.
	// Note: This only applies to externalName clusters.
	//
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/cluster/v3/cluster.proto.html#envoy-v3-api-enum-config-cluster-v3-cluster-dnslookupfamily
	// for more information.
	DNSLookupFamily ClusterDNSFamilyType `yaml:"dns-lookup-family"`
}

// NetworkParameters hold various configurable network values.
type NetworkParameters struct {
	// XffNumTrustedHops defines the number of additional ingress proxy hops from the
	// right side of the x-forwarded-for HTTP header to trust when determining the origin
	// client’s IP address.
	//
	// See https://www.envoyproxy.io/docs/envoy/v1.17.0/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto?highlight=xff_num_trusted_hops
	// for more information.
	XffNumTrustedHops uint32 `yaml:"num-trusted-hops,omitempty"`

	// Configure the port used to access the Envoy Admin interface.
	// If configured to port "0" then the admin interface is disabled.
	EnvoyAdminPort int `yaml:"admin-port,omitempty"`
}

// ListenerParameters hold various configurable listener values.
type ListenerParameters struct {
	// ConnectionBalancer. If the value is exact, the listener will use the exact connection balancer
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v2/api/v2/listener.proto#envoy-api-msg-listener-connectionbalanceconfig
	// for more information.
	ConnectionBalancer string `yaml:"connection-balancer"`
}

func (p *ListenerParameters) Validate() error {
	if p == nil {
		return nil
	}

	if p.ConnectionBalancer != "" && p.ConnectionBalancer != "exact" {
		return fmt.Errorf("invalid listener connection balancer value %q, only 'exact' connection balancing is supported for now", p.ConnectionBalancer)
	}
	return nil
}

// Parameters contains the configuration file parameters for the
// Contour ingress controller.
type Parameters struct {
	// Enable debug logging
	Debug bool

	// Kubernetes client parameters.
	InCluster  bool   `yaml:"incluster,omitempty"`
	Kubeconfig string `yaml:"kubeconfig,omitempty"`

	// Server contains parameters for the xDS server.
	Server ServerParameters `yaml:"server,omitempty"`

	// GatewayConfig contains parameters for the gateway-api Gateway that Contour
	// is configured to serve traffic.
	GatewayConfig *GatewayParameters `yaml:"gateway,omitempty"`

	// Address to be placed in status.loadbalancer field of Ingress objects.
	// May be either a literal IP address or a host name.
	// The value will be placed directly into the relevant field inside the status.loadBalancer struct.
	IngressStatusAddress string `yaml:"ingress-status-address,omitempty"`

	// AccessLogFormat sets the global access log format.
	// Valid options are 'envoy' or 'json'
	AccessLogFormat AccessLogType `yaml:"accesslog-format,omitempty"`

	// AccessLogFormatString sets the access log format when format is set to `envoy`.
	// When empty, Envoy's default format is used.
	AccessLogFormatString string `yaml:"accesslog-format-string,omitempty"`

	// AccessLogFields sets the fields that JSON logging will
	// output when AccessLogFormat is json.
	AccessLogFields AccessLogFields `yaml:"json-fields,omitempty"`

	// AccessLogLevel sets the verbosity level of the access log.
	AccessLogLevel AccessLogLevel `yaml:"accesslog-level,omitempty"`

	// TLS contains TLS policy parameters.
	TLS TLSParameters `yaml:"tls,omitempty"`

	// DisablePermitInsecure disables the use of the
	// permitInsecure field in HTTPProxy.
	DisablePermitInsecure bool `yaml:"disablePermitInsecure,omitempty"`

	// DisableAllowChunkedLength disables the RFC-compliant Envoy behavior to
	// strip the "Content-Length" header if "Transfer-Encoding: chunked" is
	// also set. This is an emergency off-switch to revert back to Envoy's
	// default behavior in case of failures. Please file an issue if failures
	// are encountered.
	// See: https://github.com/projectcontour/contour/issues/3221
	DisableAllowChunkedLength bool `yaml:"disableAllowChunkedLength,omitempty"`

	// DisableMergeSlashes disables Envoy's non-standard merge_slashes path transformation option
	// which strips duplicate slashes from request URL paths.
	DisableMergeSlashes bool `yaml:"disableMergeSlashes,omitempty"`

	// EnableExternalNameService allows processing of ExternalNameServices
	// Defaults to disabled for security reasons.
	// TODO(youngnick): put a link to the issue and CVE here.
	EnableExternalNameService bool `yaml:"enableExternalNameService,omitempty"`

	// Timeouts holds various configurable timeouts that can
	// be set in the config file.
	Timeouts TimeoutParameters `yaml:"timeouts,omitempty"`

	// Policy specifies default policy applied if not overridden by the user
	Policy PolicyParameters `yaml:"policy,omitempty"`

	// Namespace of the envoy service to inspect for Ingress status details.
	EnvoyServiceNamespace string `yaml:"envoy-service-namespace,omitempty"`

	// Name of the envoy service to inspect for Ingress status details.
	EnvoyServiceName string `yaml:"envoy-service-name,omitempty"`

	// DefaultHTTPVersions defines the default set of HTTPS
	// versions the proxy should accept. HTTP versions are
	// strings of the form "HTTP/xx". Supported versions are
	// "HTTP/1.1" and "HTTP/2".
	//
	// If this field not specified, all supported versions are accepted.
	DefaultHTTPVersions []HTTPVersionType `yaml:"default-http-versions"`

	// Cluster holds various configurable Envoy cluster values that can
	// be set in the config file.
	Cluster ClusterParameters `yaml:"cluster,omitempty"`

	// Network holds various configurable Envoy network values.
	Network NetworkParameters `yaml:"network,omitempty"`

	// Listener holds various configurable Envoy Listener values.
	Listener ListenerParameters `yaml:"listener,omitempty"`

	// RateLimitService optionally holds properties of the Rate Limit Service
	// to be used for global rate limiting.
	RateLimitService RateLimitService `yaml:"rateLimitService,omitempty"`

	// MetricsParameters holds configurable parameters for Contour and Envoy metrics.
	Metrics MetricsParameters `yaml:"metrics,omitempty"`
}

// RateLimitService defines properties of a global Rate Limit Service.
type RateLimitService struct {
	// ExtensionService identifies the extension service defining the RLS,
	// formatted as <namespace>/<name>.
	ExtensionService string `yaml:"extensionService,omitempty"`

	// Domain is passed to the Rate Limit Service.
	Domain string `yaml:"domain,omitempty"`

	// FailOpen defines whether to allow requests to proceed when the
	// Rate Limit Service fails to respond with a valid rate limit
	// decision within the timeout defined on the extension service.
	FailOpen bool `yaml:"failOpen,omitempty"`

	// EnableXRateLimitHeaders defines whether to include the X-RateLimit
	// headers X-RateLimit-Limit, X-RateLimit-Remaining, and X-RateLimit-Reset
	// (as defined by the IETF Internet-Draft linked below), on responses
	// to clients when the Rate Limit Service is consulted for a request.
	//
	// ref. https://tools.ietf.org/id/draft-polli-ratelimit-headers-03.html
	EnableXRateLimitHeaders bool `yaml:"enableXRateLimitHeaders,omitempty"`
}

// MetricsParameters defines configuration for metrics server endpoints in both
// Contour and Envoy.
type MetricsParameters struct {
	Contour MetricsServerParameters `yaml:"contour,omitempty"`
	Envoy   MetricsServerParameters `yaml:"envoy,omitempty"`
}

// MetricsServerParameters defines configuration for metrics server.
type MetricsServerParameters struct {
	// Address that metrics server will bind to.
	Address string `yaml:"address,omitempty"`

	// Port that metrics server will bind to.
	Port int `yaml:"port,omitempty"`

	// ServerCert is the file path for server certificate.
	// Optional: required only if HTTPS is used to protect the metrics endpoint.
	ServerCert string `yaml:"server-certificate-path,omitempty"`

	// ServerKey is the file path for the private key which corresponds to the server certificate.
	// Optional: required only if HTTPS is used to protect the metrics endpoint.
	ServerKey string `yaml:"server-key-path,omitempty"`

	// CABundle is the file path for CA certificate(s) used for validating the client certificate.
	// Optional: required only if client certificates shall be validated to protect the metrics endpoint.
	CABundle string `yaml:"ca-certificate-path,omitempty"`
}

func (p *MetricsParameters) Validate() error {
	if err := p.Contour.Validate(); err != nil {
		return fmt.Errorf("metrics.contour: %v", err)
	}
	if err := p.Envoy.Validate(); err != nil {
		return fmt.Errorf("metrics.envoy: %v", err)
	}

	return nil
}

func (p *MetricsServerParameters) Validate() error {
	// Check that both certificate and key are provided if either one is provided.
	if (p.ServerCert != "") != (p.ServerKey != "") {
		return fmt.Errorf("you must supply at least server-certificate-path and server-key-path or none of them")
	}

	// Optional client certificate validation can be enabled if server certificate (and consequently also key) is also provided.
	if (p.CABundle != "") && (p.ServerCert == "") {
		return fmt.Errorf("you must supply also server-certificate-path and server-key-path if setting ca-certificate-path")
	}

	return nil
}

// HasTLS returns true if parameters have been provided to enable TLS for metrics.
func (p *MetricsServerParameters) HasTLS() bool {
	return p.ServerCert != "" && p.ServerKey != ""
}

type AccessLogLevel string

func (a AccessLogLevel) Validate() error {
	return contour_api_v1alpha1.AccessLogLevel(a).Validate()
}

const LogLevelInfo AccessLogLevel = "info" // Default log level.
const LogLevelError AccessLogLevel = "error"
const LogLevelDisabled AccessLogLevel = "disabled"

// Validate verifies that the parameter values do not have any syntax errors.
func (p *Parameters) Validate() error {
	if err := p.Cluster.DNSLookupFamily.Validate(); err != nil {
		return err
	}

	if err := p.Server.XDSServerType.Validate(); err != nil {
		return err
	}

	if err := p.GatewayConfig.Validate(); err != nil {
		return err
	}

	if err := p.AccessLogFormat.Validate(); err != nil {
		return err
	}

	if err := p.AccessLogFields.Validate(); err != nil {
		return err
	}

	if err := p.AccessLogLevel.Validate(); err != nil {
		return err
	}

	if err := contour_api_v1alpha1.AccessLogFormatString(p.AccessLogFormatString).Validate(); err != nil {
		return err
	}

	if err := p.TLS.Validate(); err != nil {
		return err
	}

	if err := p.Timeouts.Validate(); err != nil {
		return err
	}

	if err := p.Policy.Validate(); err != nil {
		return err
	}

	for _, v := range p.DefaultHTTPVersions {
		if err := v.Validate(); err != nil {
			return err
		}
	}

	if err := p.Metrics.Validate(); err != nil {
		return err
	}

	return p.Listener.Validate()
}

// Defaults returns the default set of parameters.
func Defaults() Parameters {
	contourNamespace := GetenvOr("CONTOUR_NAMESPACE", "projectcontour")

	return Parameters{
		Debug:      false,
		InCluster:  false,
		Kubeconfig: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
		Server: ServerParameters{
			XDSServerType: ContourServerType,
		},
		IngressStatusAddress:      "",
		AccessLogFormat:           DEFAULT_ACCESS_LOG_TYPE,
		AccessLogFields:           DefaultFields,
		AccessLogLevel:            LogLevelInfo,
		TLS:                       TLSParameters{},
		DisablePermitInsecure:     false,
		DisableAllowChunkedLength: false,
		DisableMergeSlashes:       false,
		Timeouts: TimeoutParameters{
			// This is chosen as a rough default to stop idle connections wasting resources,
			// without stopping slow connections from being terminated too quickly.
			ConnectionIdleTimeout: "60s",
			ConnectTimeout:        "2s",
		},
		Policy: PolicyParameters{
			RequestHeadersPolicy:  HeadersPolicy{},
			ResponseHeadersPolicy: HeadersPolicy{},
			ApplyToIngress:        false,
		},
		EnvoyServiceName:      "envoy",
		EnvoyServiceNamespace: contourNamespace,
		DefaultHTTPVersions:   []HTTPVersionType{},
		Cluster: ClusterParameters{
			DNSLookupFamily: AutoClusterDNSFamily,
		},
		Network: NetworkParameters{
			XffNumTrustedHops: 0,
			EnvoyAdminPort:    9001,
		},
		Listener: ListenerParameters{
			ConnectionBalancer: "",
		},
	}
}

// Parse reads parameters from a YAML input stream. Any parameters
// not specified by the input are according to Defaults().
func Parse(in io.Reader) (*Parameters, error) {
	conf := Defaults()
	decoder := yaml.NewDecoder(in)

	decoder.KnownFields(true)

	if err := decoder.Decode(&conf); err != nil {
		// The YAML decoder will return EOF if there are
		// no YAML nodes in the results. In this case, we just
		// want to succeed and return the defaults.
		if err != io.EOF {
			return nil, fmt.Errorf("failed to parse configuration: %w", err)
		}
	}

	// Force the version string to match the lowercase version
	// constants (assuming that it will match).
	for i, v := range conf.DefaultHTTPVersions {
		conf.DefaultHTTPVersions[i] = HTTPVersionType(strings.ToLower(string(v)))
	}

	return &conf, nil
}

// GetenvOr reads an environment or return a default value
func GetenvOr(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}
