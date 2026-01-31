package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/mheap/google-contacts-backup/internal/auth"
	"github.com/mheap/google-contacts-backup/internal/contacts"
	"github.com/mheap/google-contacts-backup/internal/models"
)

var (
	outputFile string
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup Google Contacts to a JSON file",
	Long: `Download all your Google Contacts and save them to a JSON file.

The backup includes:
  - All contact fields (names, emails, phones, addresses, etc.)
  - Contact photos (as URLs - note: URLs may expire)
  - Contact groups (labels)
  - Custom fields

Examples:
  # Backup to a timestamped file (default)
  google-contacts-backup backup

  # Backup to a specific file
  google-contacts-backup backup -o my-contacts.json

  # Use a specific credentials file
  google-contacts-backup backup -c ~/my-credentials.json -o backup.json`,
	RunE: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)

	// Default filename with timestamp
	defaultOutput := fmt.Sprintf("contacts-%s.json", time.Now().Format("20060102-150405"))
	backupCmd.Flags().StringVarP(&outputFile, "output", "o", defaultOutput,
		"Output file path for the backup")
}

func runBackup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if credentials file exists
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s\n\nRun 'google-contacts-backup auth' first, or see 'google-contacts-backup --help' for setup instructions", credentialsFile)
	}

	fmt.Println("Authenticating with Google...")

	// Authenticate
	authenticator := auth.NewAuthenticator(credentialsFile)
	httpClient, err := authenticator.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println("Authentication successful!")
	fmt.Println()

	// Create contacts client
	client, err := contacts.NewClient(ctx, httpClient)
	if err != nil {
		return fmt.Errorf("failed to create contacts client: %w", err)
	}

	// Create backup file
	backup := models.NewBackupFile()

	// Fetch contact groups
	fmt.Println("Fetching contact groups...")
	groups, err := client.ListGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch contact groups: %w", err)
	}

	for _, group := range groups {
		backup.AddGroup(group)
	}
	fmt.Printf("Found %d contact groups\n", len(groups))
	fmt.Println()

	// Fetch contacts with progress bar
	fmt.Println("Fetching contacts...")

	// Create a progress bar (we'll update the max once we know the total)
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowIts(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)

	var totalKnown bool
	contactsList, err := client.ListContacts(ctx, func(current, total int) {
		if !totalKnown && total > 0 {
			bar.ChangeMax(total)
			totalKnown = true
		}
		bar.Set(current)
	})
	if err != nil {
		fmt.Println() // New line after progress bar
		return fmt.Errorf("failed to fetch contacts: %w", err)
	}

	bar.Finish()
	fmt.Println() // New line after progress bar

	for _, contact := range contactsList {
		backup.AddContact(contact)
	}

	// Save backup to file
	fmt.Printf("\nSaving backup to %s...\n", outputFile)
	if err := backup.SaveToFile(outputFile); err != nil {
		return fmt.Errorf("failed to save backup: %w", err)
	}

	// Print summary
	fmt.Println()
	fmt.Println("Backup completed successfully!")
	fmt.Println()
	fmt.Printf("  Contacts: %d\n", backup.ContactCount)
	fmt.Printf("  Groups:   %d\n", backup.GroupCount)
	fmt.Printf("  File:     %s\n", outputFile)
	fmt.Println()
	fmt.Println("Note: Contact photos are stored as URLs which may expire over time.")

	return nil
}
