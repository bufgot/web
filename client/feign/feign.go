// Package feign provides an OpenFeign-style declarative HTTP client.
//
// A feign proxy is generated for a "struct of functions": you declare a struct
// whose exported fields are function types (one per remote endpoint), then
// New[T] fills those fields with implementations that perform the HTTP call.
// Path parameters and query parameters are taken from the `path` / `query`
// struct tags (or map keys) of the method's input argument.
//
// Example:
//
//	type GetUserReq struct {
//		ID int `path:"id"`
//	}
//
//	type User struct {
//		ID   int    `json:"id"`
//		Name string `json:"name"`
//	}
//
//	type UserService struct {
//		Get    func(ctx context.Context, in *GetUserReq) (*User, error)
//		Create func(ctx context.Context, in *CreateUserReq) (*User, *feign.RespMeta, error)
//		Delete func(ctx context.Context, in *GetUserReq) error
//	}
//
//	svc, err := feign.New[UserService](&feign.Client{
//		ServiceName: "user-service",
//		Discoverer:  feign.Static(feign.InstanceOf("127.0.0.1", 8080)),
//		Mappings: map[string]string{
//			"Get":    "GET /api/users/{id}",
//			"Create": "POST /api/users",
//			"Delete": "DELETE /api/users/{id}",
//		},
//	})
//	if err != nil { ... }
//	u, err := svc.Get(ctx, &GetUserReq{ID: 42})
//
// Note on interfaces: Go's reflect package cannot add methods to a type at
// runtime, so generating an implementation of an arbitrary interface is not
// possible. The struct-of-functions style keeps the same declarative feel and
// full type safety without that limitation.
package feign

import (
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"
)

// Instance is a single upstream endpoint resolved from service discovery.
type Instance struct {
	// ID is the registry instance id (optional).
	ID string
	// IP is the host address (IPv4, IPv6 or hostname).
	IP string
	// Port is the listening port.
	Port int
}

// InstanceOf is a shorthand constructor for a single Instance.
func InstanceOf(ip string, port int) Instance {
	return Instance{IP: ip, Port: port}
}

// Discoverer resolves a service name to its available upstream instances.
//
// It is duck-typed to mirror the interface of
// github.com/bufgot/discovery.Discoverer, so the package does not need to
// import the discovery module (which would drag in etcd/viper as dependencies).
type Discoverer interface {
	Discover(serviceName string) ([]Instance, error)
}

// DiscovererFunc adapts a plain function to the Discoverer interface.
type DiscovererFunc func(serviceName string) ([]Instance, error)

// Discover implements Discoverer.
func (f DiscovererFunc) Discover(serviceName string) ([]Instance, error) {
	return f(serviceName)
}

// Static returns a Discoverer that always yields the given instances.
func Static(instances ...Instance) Discoverer {
	return DiscovererFunc(func(string) ([]Instance, error) { return instances, nil })
}

// Client configures feign proxies. A single Client can build proxies for
// several interfaces; reuse it across the application.
type Client struct {
	// ServiceName is used as the discovery key, e.g. "order-service".
	// It is combined with BaseURL when both are set.
	ServiceName string

	// Discoverer resolves ServiceName to upstream instances.
	// If nil, BaseURL must be set (or discovery is skipped).
	Discoverer Discoverer

	// Balancer selects an instance from the discovered list.
	// Defaults to RoundRobin.
	Balancer Balancer

	// BaseURL is an optional static base URL, e.g. "http://localhost:8080".
	// When Discoverer is nil it is used directly; when both are set the
	// discovered instance host/port wins.
	BaseURL string

	// Mappings maps interface method names to "METHOD /path" strings.
	// Path templates may contain {name} placeholders filled from the
	// input argument's `path` tags or map keys.
	// Example: "GetUser" -> "GET /api/users/{id}"
	Mappings map[string]string

	// Transport is the http.RoundTripper used to send requests. Useful for
	// chaining client-side middleware such as client/sign.SignTransport.
	Transport http.RoundTripper

	// Codec encodes request bodies and decodes response bodies.
	// Defaults to JSONCodec.
	Codec Codec

	// Timeout is the per-request timeout. Zero means no timeout.
	Timeout time.Duration

	// DecodeError, when set, is invoked for non-2xx responses before
	// falling back to HTTPError. It may return nil to suppress the error.
	DecodeError func(resp *http.Response, body []byte) error

	balOnce    sync.Once
	balancerVal Balancer
}

// New creates a new feign proxy for a "struct of functions" type T.
//
// T must be a struct whose exported fields are function types; each such
// field name must have an entry in c.Mappings. New returns a zero-value T
// with every function field replaced by an implementation that performs the
// corresponding HTTP call.
//
// Supported field signatures:
//
//	Get(ctx context.Context, in *In) (*Out, error)
//	Get(ctx context.Context, in *In) (*Out, RespMeta, error)
//	Get(ctx context.Context) (*Out, error)
//	Get(ctx context.Context, in *In) error
//
// The context.Context first argument is optional; when omitted the proxy uses
// context.Background().
func New[T any](c *Client) (T, error) {
	var zero T
	if c == nil {
		return zero, fmt.Errorf("feign: client is nil")
	}
	typ := reflect.TypeOf(&zero).Elem()
	if typ.Kind() != reflect.Struct {
		return zero, fmt.Errorf("feign: T must be a struct of function fields, got %s", typ.Kind())
	}
	p := &proxy{client: c}
	sv := reflect.ValueOf(&zero).Elem()
	if err := p.fill(sv); err != nil {
		return zero, err
	}
	return zero, nil
}
