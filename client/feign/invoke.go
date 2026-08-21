package feign

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
)

var (
	ctxType  = reflect.TypeOf((*context.Context)(nil)).Elem()
	errType  = reflect.TypeOf((*error)(nil)).Elem()
	metaType = reflect.TypeOf(RespMeta{})
)

// proxy holds the client configuration shared by all generated functions.
type proxy struct {
	client *Client
}

// fill replaces every exported function field of sv with an implementation
// that performs the corresponding HTTP call (looked up in client.Mappings).
func (p *proxy) fill(sv reflect.Value) error {
	t := sv.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		ft := f.Type
		if ft.Kind() != reflect.Func {
			continue
		}
		name := f.Name
		fn := reflect.MakeFunc(ft, func(args []reflect.Value) []reflect.Value {
			return p.call(name, ft, args)
		})
		sv.Field(i).Set(fn)
	}
	return nil
}

// call implements a single function field through the HTTP client.
func (p *proxy) call(name string, ft reflect.Type, args []reflect.Value) []reflect.Value {
	ctx := context.Background()
	var in any
	hasIn := false

	for i := 0; i < ft.NumIn(); i++ {
		at := ft.In(i)
		if at == ctxType || at.Implements(ctxType) {
			if at.Kind() == reflect.Interface && !args[i].IsNil() {
				if v, ok := args[i].Interface().(context.Context); ok {
					ctx = v
				}
			}
			continue
		}
		if !hasIn && args[i].IsValid() && !isNilValue(args[i]) {
			in = args[i].Interface()
			hasIn = true
		}
	}

	body, meta, err := p.do(ctx, name, in, hasIn)
	if err != nil {
		return p.errorResults(ft, err)
	}
	return p.successResults(ft, body, meta)
}

func (p *proxy) do(ctx context.Context, methodName string, in any, hasIn bool) ([]byte, *RespMeta, error) {
	req, err := p.buildRequest(ctx, methodName, in, hasIn)
	if err != nil {
		return nil, nil, err
	}
	resp, err := p.client.do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("feign: %s: %w", methodName, err)
	}
	meta := &RespMeta{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header.Clone(),
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, meta, fmt.Errorf("feign: %s: read response: %w", methodName, err)
	}
	if resp.StatusCode >= 400 {
		if p.client.DecodeError != nil {
			if derr := p.client.DecodeError(resp, body); derr != nil {
				return nil, meta, derr
			}
			// DecodeError returned nil: treat the response as handled.
			return body, meta, nil
		}
		return nil, meta, &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       body,
			Message:    decodeErrorMessage(body),
		}
	}
	return body, meta, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	transport := c.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	hc := &http.Client{Transport: transport, Timeout: c.Timeout}
	return hc.Do(req)
}

func (p *proxy) buildRequest(ctx context.Context, methodName string, in any, hasIn bool) (*http.Request, error) {
	mapping, ok := p.client.Mappings[methodName]
	if !ok {
		return nil, fmt.Errorf("feign: no mapping for method %s", methodName)
	}
	method, tmpl, err := parseMapping(mapping)
	if err != nil {
		return nil, fmt.Errorf("feign: method %s: %w", methodName, err)
	}

	path := tmpl
	if hasIn && in != nil {
		path = applyPathParams(tmpl, in)
	}
	u, err := p.client.resolveURL(path)
	if err != nil {
		return nil, fmt.Errorf("feign: method %s: %w", methodName, err)
	}

	var bodyReader io.Reader
	if method != http.MethodGet && method != http.MethodHead && hasIn && in != nil {
		data, err := p.client.codec().Encode(in)
		if err != nil {
			return nil, fmt.Errorf("feign: method %s: encode body: %w", methodName, err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("feign: method %s: new request: %w", methodName, err)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", p.client.codec().ContentType())
	}
	req.Header.Set("Accept", p.client.codec().ContentType())

	if hasIn && in != nil {
		appendQuery(req, in)
	}
	return req, nil
}

func (c *Client) codec() Codec {
	if c.Codec != nil {
		return c.Codec
	}
	return JSONCodec{}
}

// successResults builds the return values for a successful call.
func (p *proxy) successResults(mt reflect.Type, body []byte, meta *RespMeta) []reflect.Value {
	switch mt.NumOut() {
	case 1:
		return []reflect.Value{reflect.Zero(errType)}
	case 2:
		out := decodeBody(p.client.codec(), mt.Out(0), body)
		return []reflect.Value{out, reflect.Zero(errType)}
	case 3:
		out := decodeBody(p.client.codec(), mt.Out(0), body)
		metaVal := reflect.ValueOf(meta)
		if meta == nil {
			metaVal = reflect.Zero(metaType)
		}
		return []reflect.Value{out, metaVal, reflect.Zero(errType)}
	default:
		panic(fmt.Sprintf("feign: unsupported method signature with %d return values", mt.NumOut()))
	}
}

// errorResults builds the return values for a failed call.
func (p *proxy) errorResults(mt reflect.Type, err error) []reflect.Value {
	errVal := reflect.ValueOf(err)
	if !errVal.IsValid() {
		errVal = reflect.Zero(errType)
	}
	switch mt.NumOut() {
	case 1:
		return []reflect.Value{errVal}
	case 2:
		return []reflect.Value{reflect.Zero(mt.Out(0)), errVal}
	case 3:
		return []reflect.Value{reflect.Zero(mt.Out(0)), reflect.Zero(metaType), errVal}
	default:
		panic(fmt.Sprintf("feign: unsupported method signature with %d return values", mt.NumOut()))
	}
}

// decodeBody converts a response body into the declared output type.
func decodeBody(codec Codec, outType reflect.Type, body []byte) reflect.Value {
	switch outType.Kind() {
	case reflect.String:
		return reflect.ValueOf(string(body)).Convert(outType)
	case reflect.Slice:
		if outType.Elem().Kind() == reflect.Uint8 {
			return reflect.ValueOf(body).Convert(outType)
		}
	}
	elem := outType
	if outType.Kind() == reflect.Pointer {
		elem = outType.Elem()
	}
	out := reflect.New(elem)
	if len(body) > 0 {
		_ = codec.Decode(body, out.Interface())
	}
	if outType.Kind() == reflect.Pointer {
		return out
	}
	return out.Elem()
}

func isNilValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	default:
		return false
	}
}
