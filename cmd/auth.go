package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mheap/google-contacts-backup/internal/auth"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Google",
	Long: `Start the OAuth2 authentication flow to authorize access to your Google Contacts.

This command will:
  1. Start a local HTTP server to receive the OAuth callback
  2. Open your default browser to Google's consent page
  3. Wait for you to authorize the application
  4. Save the access and refresh tokens locally

The tokens are cached in ~/.google-contacts-backup/token.json and will be
automatically refreshed when they expire.

You only need to run this command once, or when you want to re-authenticate
with a different Google account.

Examples:
  # Authenticate with default credentials file
  google-contacts-backup auth

  # Authenticate with a custom credentials file
  google-contacts-backup auth -c ~/my-credentials.json`,
	RunE: runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if credentials file exists
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		return fmt.Errorf(`credentials file not found: %s

Please download OAuth credentials from Google Cloud Console:
  1. Go to https://console.cloud.google.com/
  2. Create or select a project
  3. Enable the People API
  4. Create OAuth 2.0 credentials (Desktop application)
  5. Download the credentials JSON file
  6. Save it to: %s
     (or specify a custom path with --credentials)`, credentialsFile, getDefaultCredentialsPath())
	}

	fmt.Println("Starting Google authentication...")
	fmt.Println()

	// Authenticate
	authenticator := auth.NewAuthenticator(credentialsFile)
	_, err := authenticator.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println()
	fmt.Println("Authentication successful!")
	fmt.Println()
	fmt.Println("Your credentials have been saved and will be used automatically")
	fmt.Println("for future backup and restore operations.")
	fmt.Println()
	fmt.Println("You can now run:")
	fmt.Println("  google-contacts-backup backup    # to backup your contacts")
	fmt.Println("  google-contacts-backup restore   # to restore from a backup")

	return nil
}
