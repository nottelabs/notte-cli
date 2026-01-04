package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/auth"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  "Login, logout, and check authentication status.",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with notte.cc",
	Long: `Open browser to authenticate with your notte.cc API key.

The API key will be stored securely in your system keychain.
Get your API key from https://notte.cc/settings`,
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
	fmt.Println("Opening browser for authentication...")

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

	fmt.Println("API key stored successfully in keychain.")
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	if err := auth.DeleteKeyringAPIKey(); err != nil {
		return fmt.Errorf("failed to remove API key: %w", err)
	}

	fmt.Println("API key removed from keychain.")
	return nil
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
