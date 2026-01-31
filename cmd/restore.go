package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/mheap/google-contacts-backup/internal/auth"
	"github.com/mheap/google-contacts-backup/internal/contacts"
	"github.com/mheap/google-contacts-backup/internal/models"
)

var (
	inputFile   string
	skipConfirm bool
)

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore Google Contacts from a JSON backup file",
	Long: `Restore your Google Contacts from a previously created backup file.

WARNING: This operation is DESTRUCTIVE! It will:
  1. DELETE ALL existing contacts in your Google account
  2. DELETE ALL user-created contact groups (labels)
  3. Recreate contact groups from the backup
  4. Recreate all contacts from the backup

System groups (My Contacts, Starred, etc.) are preserved but their
membership is reset.

It is STRONGLY recommended to create a fresh backup before restoring:
  google-contacts-backup backup -o pre-restore-backup.json

Examples:
  # Restore from a backup file (will prompt for confirmation)
  google-contacts-backup restore -i my-contacts.json

  # Restore without confirmation prompt (for scripting)
  google-contacts-backup restore -i my-contacts.json --confirm

  # Use a specific credentials file
  google-contacts-backup restore -c ~/creds.json -i backup.json`,
	RunE: runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().StringVarP(&inputFile, "input", "i", "",
		"Input backup file path (required)")
	restoreCmd.MarkFlagRequired("input")

	restoreCmd.Flags().BoolVar(&skipConfirm, "confirm", false,
		"Skip confirmation prompt (use with caution!)")
}

func runRestore(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", inputFile)
	}

	// Load and validate backup file
	fmt.Printf("Loading backup file: %s\n", inputFile)
	backup, err := models.LoadBackupFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to load backup: %w", err)
	}

	fmt.Println()
	fmt.Println("Backup file information:")
	fmt.Printf("  Version:    %s\n", backup.Version)
	fmt.Printf("  Created:    %s\n", backup.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Contacts:   %d\n", backup.ContactCount)
	fmt.Printf("  Groups:     %d\n", backup.GroupCount)
	fmt.Println()

	// Check if credentials file exists
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s\n\nRun 'google-contacts-backup auth' first, or see 'google-contacts-backup --help' for setup instructions", credentialsFile)
	}

	// Confirm with user unless --confirm flag is set
	if !skipConfirm {
		fmt.Println("WARNING: This will DELETE ALL existing contacts and groups!")
		fmt.Println("It is recommended to create a backup first:")
		fmt.Println("  google-contacts-backup backup -o pre-restore-backup.json")
		fmt.Println()
		fmt.Print("Are you sure you want to continue? (yes/no): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" && response != "y" {
			fmt.Println("Restore cancelled.")
			return nil
		}
		fmt.Println()
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

	// Step 1: Delete all existing contacts
	fmt.Println("Step 1/4: Deleting existing contacts...")
	deleteContactsBar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Deleting contacts"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)

	var deleteTotal int
	err = client.DeleteAllContacts(ctx, func(deleted, total int) {
		if deleteTotal == 0 && total > 0 {
			deleteContactsBar.ChangeMax(total)
			deleteTotal = total
		}
		deleteContactsBar.Set(deleted)
	})
	deleteContactsBar.Finish()
	fmt.Println()

	if err != nil {
		return fmt.Errorf("failed to delete contacts: %w", err)
	}

	if deleteTotal > 0 {
		fmt.Printf("Deleted %d contacts\n", deleteTotal)
	} else {
		fmt.Println("No existing contacts to delete")
	}
	fmt.Println()

	// Step 2: Delete user-created groups
	fmt.Println("Step 2/4: Deleting existing contact groups...")
	deleteGroupsBar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Deleting groups"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)

	var deleteGroupTotal int
	err = client.DeleteUserGroups(ctx, func(deleted, total int) {
		if deleteGroupTotal == 0 && total > 0 {
			deleteGroupsBar.ChangeMax(total)
			deleteGroupTotal = total
		}
		deleteGroupsBar.Set(deleted)
	})
	deleteGroupsBar.Finish()
	fmt.Println()

	if err != nil {
		return fmt.Errorf("failed to delete groups: %w", err)
	}

	if deleteGroupTotal > 0 {
		fmt.Printf("Deleted %d groups\n", deleteGroupTotal)
	} else {
		fmt.Println("No user-created groups to delete")
	}
	fmt.Println()

	// Step 3: Recreate contact groups
	userGroups := backup.GetUserGroups()
	groupMap := make(map[string]string)

	if len(userGroups) > 0 {
		fmt.Println("Step 3/4: Creating contact groups...")
		createGroupsBar := progressbar.NewOptions(len(userGroups),
			progressbar.OptionSetDescription("Creating groups"),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(40),
			progressbar.OptionThrottle(100*time.Millisecond),
			progressbar.OptionFullWidth(),
			progressbar.OptionSetRenderBlankState(true),
		)

		groupMap, err = client.CreateGroups(ctx, userGroups, func(created, total int) {
			createGroupsBar.Set(created)
		})
		createGroupsBar.Finish()
		fmt.Println()

		if err != nil {
			return fmt.Errorf("failed to create groups: %w", err)
		}

		fmt.Printf("Created %d groups\n", len(groupMap))
	} else {
		fmt.Println("Step 3/4: No user-created groups to restore")
	}
	fmt.Println()

	// Step 4: Recreate contacts
	if len(backup.Contacts) > 0 {
		fmt.Println("Step 4/4: Creating contacts...")
		createContactsBar := progressbar.NewOptions(len(backup.Contacts),
			progressbar.OptionSetDescription("Creating contacts"),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(40),
			progressbar.OptionThrottle(100*time.Millisecond),
			progressbar.OptionFullWidth(),
			progressbar.OptionSetRenderBlankState(true),
		)

		err = client.CreateContacts(ctx, backup.Contacts, groupMap, func(created, total int) {
			createContactsBar.Set(created)
		})
		createContactsBar.Finish()
		fmt.Println()

		if err != nil {
			return fmt.Errorf("failed to create contacts: %w", err)
		}

		fmt.Printf("Created %d contacts\n", len(backup.Contacts))
	} else {
		fmt.Println("Step 4/4: No contacts to restore")
	}

	// Print summary
	fmt.Println()
	fmt.Println("Restore completed successfully!")
	fmt.Println()
	fmt.Printf("  Contacts restored: %d\n", len(backup.Contacts))
	fmt.Printf("  Groups restored:   %d\n", len(groupMap))
	fmt.Println()
	fmt.Println("Note: Contact photos were not restored (API limitation).")
	fmt.Println("Photo URLs in the backup may have expired.")

	return nil
}
