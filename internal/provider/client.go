package provider

// Client is a stub for the Hashnode GraphQL provider client.

type Client struct {
	Token string
}

func NewClient(token string) *Client {
	return &Client{Token: token}
}

// TODO: add methods to query posts, create/update posts, etc.
