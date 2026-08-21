package feign

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

// parseMapping parses a "METHOD /path" mapping string into its parts.
// The method is case-insensitive and normalized to upper case.
func parseMapping(s string) (method, path string, err error) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return "", "", fmt.Errorf("feign: invalid mapping %q, want \"METHOD /path\"", s)
	}
	method = strings.ToUpper(fields[0])
	path = fields[1]
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions:
	default:
		return "", "", fmt.Errorf("feign: unsupported HTTP method %q", method)
	}
	if !strings.HasPrefix(path, "/") {
		return "", "", fmt.Errorf("feign: path %q must start with /", path)
	}
	return method, path, nil
}

// applyPathParams fills {name} placeholders in the path template.
// Values come from the input argument's `path` struct tags or map keys.
// Missing placeholders are left untouched.
func applyPathParams(tmpl string, in any) string {
	params := pathParams(in)
	if len(params) == 0 {
		return tmpl
	}
	result := tmpl
	for {
		start := strings.Index(result, "{")
		if start < 0 {
			return result
		}
		rel := strings.Index(result[start:], "}")
		if rel < 0 {
			return result
		}
		end := start + rel
		key := result[start+1 : end]
		val, ok := params[key]
		if !ok {
			// leave unknown placeholder as-is and continue after it
			result = result[:end+1]
			continue
		}
		result = result[:start] + url.PathEscape(val) + result[end+1:]
	}
}

// pathParams extracts {name} -> value from the input argument.
func pathParams(in any) map[string]string {
	out := map[string]string{}
	rv := reflect.ValueOf(in)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return out
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return out
		}
		for _, k := range rv.MapKeys() {
			mi := rv.MapIndex(k)
			if mi.IsValid() && !isZeroValue(mi) {
				out[k.String()] = fmt.Sprint(mi.Interface())
			}
		}
	case reflect.Struct:
		t := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			key := f.Tag.Get("path")
			if key == "" {
				continue
			}
			fv := rv.Field(i)
			if fv.IsValid() && !isZeroValue(fv) {
				out[key] = fmt.Sprint(fv.Interface())
			}
		}
	}
	return out
}

// appendQuery adds query parameters to the request URL from the input
// argument's `query` struct tags (or map keys).
func appendQuery(req *http.Request, in any) {
	rv := reflect.ValueOf(in)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	q := req.URL.Query()
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return
		}
		for _, k := range rv.MapKeys() {
			mi := rv.MapIndex(k)
			if mi.IsValid() && !isZeroValue(mi) {
				q.Set(k.String(), fmt.Sprint(mi.Interface()))
			}
		}
	case reflect.Struct:
		t := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			key := f.Tag.Get("query")
			if key == "" {
				continue
			}
			fv := rv.Field(i)
			if fv.IsValid() && !isZeroValue(fv) {
				q.Set(key, fmt.Sprint(fv.Interface()))
			}
		}
	}
	req.URL.RawQuery = q.Encode()
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer, reflect.Slice, reflect.Map,
		reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return v.IsNil()
	default:
		return v.IsZero()
	}
}
