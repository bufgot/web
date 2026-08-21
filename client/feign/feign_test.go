package feign_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bufgot/web/client/feign"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type GetUserReq struct {
	ID   int    `path:"id"`
	Verbose bool `query:"verbose"`
}

type CreateUserReq struct {
	Name string `json:"name"`
}

type UserService struct {
	Get    func(ctx context.Context, in *GetUserReq) (*User, error)
	Create func(ctx context.Context, in *CreateUserReq) (*User, *feign.RespMeta, error)
	List   func(ctx context.Context) ([]User, error)
	Raw    func(ctx context.Context) ([]byte, error)
	Delete func(ctx context.Context, in *GetUserReq) error
}

func newTestClient(base string) *feign.Client {
	return &feign.Client{
		ServiceName: "user-service",
		BaseURL:     base,
		Mappings: map[string]string{
			"Get":    "GET /api/users/{id}",
			"Create": "POST /api/users",
			"List":   "GET /api/users",
			"Raw":    "GET /api/raw",
			"Delete": "DELETE /api/users/{id}",
		},
	}
}

func TestFeignGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/users/42" {
			t.Fatalf("path = %s, want /api/users/42", r.URL.Path)
		}
		if r.URL.Query().Get("verbose") != "true" {
			t.Fatalf("verbose query missing: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"name":"marvis"}`))
	}))
	defer srv.Close()

	svc, err := feign.New[UserService](newTestClient(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	u, err := svc.Get(context.Background(), &GetUserReq{ID: 42, Verbose: true})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if u.ID != 42 || u.Name != "marvis" {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestFeignCreateWithMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var in CreateUserReq
		if err := json.Unmarshal(body, &in); err != nil {
			t.Fatalf("bad body: %v", err)
		}
		if in.Name != "marvis" {
			t.Fatalf("name = %q, want marvis", in.Name)
		}
		w.Header().Set("X-Trace", "abc123")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":7,"name":"marvis"}`))
	}))
	defer srv.Close()

	svc, err := feign.New[UserService](newTestClient(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	u, meta, err := svc.Create(context.Background(), &CreateUserReq{Name: "marvis"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID != 7 {
		t.Fatalf("unexpected id: %d", u.ID)
	}
	if meta.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", meta.StatusCode)
	}
	if meta.Header.Get("X-Trace") != "abc123" {
		t.Fatalf("trace header missing")
	}
}

func TestFeignListSliceAndRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users":
			_, _ = w.Write([]byte(`[{"id":1,"name":"a"},{"id":2,"name":"b"}]`))
		case "/api/raw":
			_, _ = w.Write([]byte(`raw-bytes`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc, err := feign.New[UserService](newTestClient(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].ID != 1 || list[1].Name != "b" {
		t.Fatalf("unexpected list: %+v", list)
	}

	raw, err := svc.Raw(context.Background())
	if err != nil {
		t.Fatalf("Raw: %v", err)
	}
	if string(raw) != "raw-bytes" {
		t.Fatalf("raw = %q", string(raw))
	}
}

func TestFeignHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"user not found"}`))
	}))
	defer srv.Close()

	svc, err := feign.New[UserService](newTestClient(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = svc.Get(context.Background(), &GetUserReq{ID: 404})
	if err == nil {
		t.Fatal("expected error")
	}
	var he *feign.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("err = %v, want *feign.HTTPError", err)
	}
	if he.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", he.StatusCode)
	}
	if !strings.Contains(he.Message, "user not found") {
		t.Fatalf("message = %q", he.Message)
	}
}

func TestFeignDeleteErrorOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/users/9" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	svc, err := feign.New[UserService](newTestClient(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := svc.Delete(context.Background(), &GetUserReq{ID: 9}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestFeignMissingMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc, err := feign.New[UserService](&feign.Client{
		BaseURL: srv.URL,
		Mappings: map[string]string{
			"Get": "GET /api/users/{id}",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = svc.List(context.Background())
	if err == nil {
		t.Fatal("expected missing-mapping error")
	}
	if !strings.Contains(err.Error(), "no mapping") {
		t.Fatalf("err = %v", err)
	}
}

func TestFeignRoundRobin(t *testing.T) {
	seen := map[string]bool{}
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		seen["srv1"] = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		seen["srv2"] = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()

	host1 := strings.TrimPrefix(srv1.URL, "http://")
	host2 := strings.TrimPrefix(srv2.URL, "http://")
	svc, err := feign.New[UserService](&feign.Client{
		ServiceName: "rr",
		Discoverer: feign.DiscovererFunc(func(string) ([]feign.Instance, error) {
			// derive host/port from httptest URLs
			h1, p1 := splitHostPort(host1)
			h2, p2 := splitHostPort(host2)
			return []feign.Instance{
				{IP: h1, Port: p1},
				{IP: h2, Port: p2},
			}, nil
		}),
		Mappings: map[string]string{
			"Get": "GET /x",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := svc.Get(context.Background(), &GetUserReq{}); err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
	}
	if !seen["srv1"] || !seen["srv2"] {
		t.Fatalf("round robin did not hit both: %v", seen)
	}
}

func splitHostPort(addr string) (string, int) {
	idx := strings.LastIndex(addr, ":")
	var port int
	_, _ = fmt.Sscanf(addr[idx+1:], "%d", &port)
	return addr[:idx], port
}

func TestFeignDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`"down"`))
	}))
	defer srv.Close()

	svc, err := feign.New[UserService](&feign.Client{
		BaseURL: srv.URL,
		Mappings: map[string]string{"Get": "GET /x"},
		DecodeError: func(_ *http.Response, body []byte) error {
			return fmt.Errorf("custom upstream error: %s", body)
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = svc.Get(context.Background(), &GetUserReq{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "custom upstream error") {
		t.Fatalf("err = %v", err)
	}
}
