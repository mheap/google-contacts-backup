// Package cmd contains the CLI commands for google-contacts-backup.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"

	// credentialsFile is the path to the OAuth credentials file
	credentialsFile string
)

// getDefaultCredentialsPath returns the default path for credentials.json
// using XDG_CONFIG_HOME if set, otherwise ~/.config
func getDefaultCredentialsPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if we can't get home
			return "credentials.json"
		}
		configDir = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configDir, "google-contacts-backup", "credentials.json")
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "google-contacts-backup",
	Short: "Backup and restore Google Contacts",
	Long: `A CLI tool to backup your Google Contacts to a JSON file and restore them.

This tool uses OAuth2 to authenticate with Google and access your contacts
via the People API. You need to provide a credentials.json file downloaded
from the Google Cloud Console.

Setup:
  1. Go to https://console.cloud.google.com/
  2. Create a new project (or select an existing one)
  3. Enable the People API
  4. Create OAuth 2.0 credentials (Desktop application)
  5. Download the credentials JSON file
  6. Save it to $XDG_CONFIG_HOME/google-contacts-backup/credentials.json
     (or ~/.config/google-contacts-backup/credentials.json)

Examples:
  # First, authenticate with Google
  google-contacts-backup auth

  # Backup contacts to a timestamped file
  google-contacts-backup backup

  # Backup to a specific file
  google-contacts-backup backup -o my-contacts.json

  # Restore contacts from a backup (destructive!)
  google-contacts-backup restore -i my-contacts.json

Note: The restore command will DELETE ALL existing contacts before restoring.
Always create a fresh backup before restoring!`,
	Version: Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	defaultCreds := getDefaultCredentialsPath()
	rootCmd.PersistentFlags().StringVarP(&credentialsFile, "credentials", "c", defaultCreds,
		"Path to the OAuth credentials JSON file from Google Cloud Console")
}
