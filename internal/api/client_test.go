package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestAuthTransportBearerHeader(t *testing.T) {
	for _, token := range []string{" test-token ", "Bearer test-token", "bearer test-token"} {
		t.Run(token, func(t *testing.T) {
			original, _ := http.NewRequest(http.MethodPost, Endpoint, strings.NewReader("{}"))
			original.Header.Set("Authorization", "original-header")
			transport := &authTransport{
				token: token,
				next: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
						t.Errorf("Authorization = %q", got)
					}
					if got := req.Header.Get("Accept"); got != "application/json" {
						t.Errorf("Accept = %q", got)
					}
					return &http.Response{
						StatusCode: 200,
						Header: http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(`{"data":{}}`)),
					}, nil
				}),
			}
			resp, err := transport.RoundTrip(original)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if got := original.Header.Get("Authorization"); got != "original-header" {
				t.Errorf("mutated original request header: %q", got)
			}
		})
	}
}

func TestAuthTransportRejectsHTML(t *testing.T) {
	transport := &authTransport{
		token: "test-token",
		next: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body: io.NopCloser(strings.NewReader("<html>error</html>")),
			}, nil
		}),
	}
	request, _ := http.NewRequest(http.MethodPost, Endpoint, nil)
	resp, err := transport.RoundTrip(request)
	if resp != nil || err == nil || !strings.Contains(err.Error(), "not GraphQL JSON") {
		t.Fatalf("expected actionable HTML response error; got response=%v err=%v", resp, err)
	}
}

func TestEndpointIsCurrent(t *testing.T) {
	if Endpoint != "https://gql-beta.hashnode.com/" {
		t.Fatalf("unexpected API endpoint: %s", Endpoint)
	}
}
