package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Khan/genqlient/graphql"
)

// Endpoint is the current Hashnode GraphQL API endpoint. gql.hashnode.com
// redirects to an HTML deprecation notice instead of serving GraphQL.
const Endpoint = "https://gql-beta.hashnode.com/"

// NewClient returns a GraphQL client with the expected Bearer authorization.
func NewClient(token string) graphql.Client {
	return graphql.NewClient(Endpoint, &http.Client{
		Transport: &authTransport{token: token, next: http.DefaultTransport},
	})
}

type authTransport struct {
	token string
	next  http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := strings.TrimSpace(t.token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[len("bearer "):])
	}

	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+token)
	clone.Header.Set("Accept", "application/json")

	resp, err := t.next.RoundTrip(clone)
	if err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		resp.Body.Close()
		return nil, fmt.Errorf("Hashnode API returned HTML (HTTP %d), not GraphQL JSON; verify the endpoint and publication access", resp.StatusCode)
	}
	return resp, nil
}
