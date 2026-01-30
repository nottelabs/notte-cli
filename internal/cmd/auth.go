package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/auth"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  "Login, logout, and check authentication status.",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with notte.cc",
	Long: `Open browser to authenticate with Notte.

You can authenticate in two ways:
  1. Sign in with Notte Console (recommended) - automatically fetches your API key
  2. Enter API key manually - paste your key from console.notte.cc/apikeys

The API key will be stored securely in your system keychain.`,
	RunE: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored credentials",
	RunE:  runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runAuthStatus,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	PrintInfo("Opening browser for authentication...")

	server := auth.NewSetupServer()

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	result, err := server.Start(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if result.Error != nil {
		return result.Error
	}

	return PrintResult("API key stored successfully in keychain.", map[string]any{
		"authenticated": true,
		"source":        "keychain",
	})
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	if err := auth.DeleteKeyringAPIKey(); err != nil {
		return fmt.Errorf("failed to remove API key: %w", err)
	}

	return PrintResult("API key removed from keychain.", map[string]any{
		"authenticated": false,
	})
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	key, source, err := auth.GetAPIKey("")
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	// Mask key for display (handle short keys safely)
	var masked string
	if len(key) < 12 {
		masked = "****"
	} else {
		masked = key[:8] + "..." + key[len(key)-4:]
	}

	formatter := GetFormatter()
	data := map[string]any{
		"Authenticated": "yes",
		"Source":        string(source),
		"API Key":       masked,
	}

	return formatter.Print(data)
}
