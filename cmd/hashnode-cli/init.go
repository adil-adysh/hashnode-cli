package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/spf13/cobra"

	// Update this import path to match your go.mod module name
	"adil-adysh/hashnode-cli/internal/api"
	"adil-adysh/hashnode-cli/internal/config"
)

// authedTransport injects the Personal Access Token into every request
type authedTransport struct {
	token   string
	wrapped http.RoundTripper
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", t.token)
	return t.wrapped.RoundTrip(req)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Setup hashnode-cli with your account",
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)

		// 1. Get Token Input (from ENV or prompt)
		token := os.Getenv("HASHNODE_TOKEN")
		if token == "" {
			token = os.Getenv("HASHNODE_API_KEY")
		}
		if token == "" {
			fmt.Print("🔑 Enter your Hashnode Personal Access Token: ")
			token, _ = reader.ReadString('\n')
			token = strings.TrimSpace(token)
		}
		if token == "" {
			fmt.Println("❌ Token cannot be empty.")
			os.Exit(1)
		}

		// 2. Setup the API Client
		httpClient := &http.Client{
			Transport: &authedTransport{
				token:   token,
				wrapped: http.DefaultTransport,
			},
		}
		client := graphql.NewClient("https://gql.hashnode.com", httpClient)

		// 3. Verify Token via API
		fmt.Println("⏳ Verifying token and fetching user details...")

		resp, err := api.GetMe(context.Background(), client)
		if err != nil {
			fmt.Printf("❌ API Error: %v\n", err)
			fmt.Println("   (Check your internet connection or if the token is valid)")
			os.Exit(1)
		}

		user := resp.Me
		if user.Username == "" {
			fmt.Println("❌ Invalid Token. API returned no username.")
			os.Exit(1)
		}

		fmt.Printf("✅ Authenticated as: @%s\n", user.Username)

		// 4. Save all Publications to config
		pubs := user.Publications.Edges
		if len(pubs) == 0 {
			fmt.Println("❌ No publications found for this account.")
			os.Exit(1)
		}

		var allPubs []config.Publication
		for _, edge := range pubs {
			allPubs = append(allPubs, config.Publication{
				ID:    edge.Node.Id,
				Title: edge.Node.Title,
				URL:   edge.Node.Url,
			})
		}

		cfg := config.Config{
			Publications: allPubs,
			Token:        token,
		}

		if err := cfg.Save(); err != nil {
			fmt.Printf("❌ Failed to write config: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n🎉 Success! hashnode-cli initialized.")
		fmt.Printf("   Config saved to: %s\n", config.ConfigPath())
		fmt.Println("   ⚠️  WARNING: This file contains your token. Keep it safe!")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
