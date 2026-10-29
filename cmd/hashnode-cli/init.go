package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	// Update this import path to match your go.mod module name
	"adil-adysh/hashnode-cli/internal/api"
	"adil-adysh/hashnode-cli/internal/cli/output"
	"adil-adysh/hashnode-cli/internal/config"
	"adil-adysh/hashnode-cli/internal/state"
)

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
		// 1a. Try loading token from user config in home dir
		if token == "" {
			if cfg, err := config.Load(); err == nil {
				if cfg != nil && cfg.Token != "" {
					token = cfg.Token
				}
			}
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
		client := api.NewClient(token)

		// 3. Verify Token via API
		output.Info("⏳ Verifying token and fetching user details...\n")

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

		output.Success("✅ Authenticated as: @%s\n", user.Username)

		// 4. Let user select a single publication (one blog per repo)
		pubs := user.Publications.Edges
		if len(pubs) == 0 {
			fmt.Println("❌ No publications found for this account.")
			os.Exit(1)
		}

		output.Info("\nYour Publications:\n")
		for i, edge := range pubs {
			output.Info("  [%d] %s (ID: %s)\n", i+1, edge.Node.Title, edge.Node.Id)
		}

		var selected int
		for {
			output.Info("Select a publication [1-%d]: ", len(pubs))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			n, err := fmt.Sscanf(input, "%d", &selected)
			if err == nil && n == 1 && selected >= 1 && selected <= len(pubs) {
				break
			}
			fmt.Println("Invalid selection. Please enter a number from the list.")
		}

		pubNode := pubs[selected-1].Node
		output.Info("📂 Selected Publication: '%s' (ID: %s)\n", pubNode.Title, pubNode.Id)

		// 5. Ensure repo-level .hashnode state directory and blog.yml
		if err := state.EnsureStateDir(); err != nil {
			output.Error("❌ Failed to create state dir: %v\n", err)
			os.Exit(1)
		}

		blogPath := state.StatePath("blog.yml")

		if _, err := os.Stat(blogPath); err == nil {
			output.Error("❌ Repository already initialized: %s exists\n", blogPath)
			os.Exit(1)
		} else if !os.IsNotExist(err) {
			output.Error("❌ Failed to check state: %v\n", err)
			os.Exit(1)
		}

		// Compose blog.yml content (system-owned)
		blog := struct {
			PublicationID   string `yaml:"publication_id"`
			PublicationSlug string `yaml:"publication_slug"`
			Title           string `yaml:"title"`
			OwnerUsername   string `yaml:"owner_username"`
		}{
			PublicationID:   pubNode.Id,
			PublicationSlug: pubNode.Url,
			Title:           pubNode.Title,
			OwnerUsername:   user.Username,
		}

		data, err := yaml.Marshal(blog)
		if err != nil {
			output.Error("❌ Failed to marshal blog state: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(blogPath, data, state.FilePerm); err != nil {
			output.Error("❌ Failed to write %s: %v\n", blogPath, err)
			os.Exit(1)
		}

		// 6. Save token to user config (home) for subsequent API calls (non-authoritative)
		cfg := config.Config{Publications: nil, Token: token}
		if err := cfg.Save(); err != nil {
			output.Error("⚠️  Failed to write home config: %v\n", err)
		}

		output.Success("\n🎉 Success! repository initialized for a single Hashnode publication.\n")
		output.Info("   State written to: %s\n", blogPath)
		output.Info("   ⚠️  WARNING: files under .hashnode/ are CLI-owned; do not edit them by hand.\n")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
