// Package models defines data structures for backup files.
package models

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"google.golang.org/api/people/v1"
)

const (
	// BackupVersion is the current version of the backup file format
	BackupVersion = "1.0"
)

// BackupFile represents the complete backup data structure.
type BackupFile struct {
	// Version of the backup file format
	Version string `json:"version"`

	// CreatedAt is the timestamp when the backup was created
	CreatedAt time.Time `json:"created_at"`

	// ContactCount is the total number of contacts in the backup
	ContactCount int `json:"contact_count"`

	// GroupCount is the total number of contact groups in the backup
	GroupCount int `json:"group_count"`

	// Contacts contains all backed up contact data
	Contacts []*people.Person `json:"contacts"`

	// Groups contains all backed up contact group data
	Groups []*people.ContactGroup `json:"groups"`
}

// NewBackupFile creates a new backup file with the current timestamp.
func NewBackupFile() *BackupFile {
	return &BackupFile{
		Version:   BackupVersion,
		CreatedAt: time.Now().UTC(),
		Contacts:  make([]*people.Person, 0),
		Groups:    make([]*people.ContactGroup, 0),
	}
}

// AddContact adds a contact to the backup and updates the count.
func (b *BackupFile) AddContact(contact *people.Person) {
	b.Contacts = append(b.Contacts, contact)
	b.ContactCount = len(b.Contacts)
}

// AddGroup adds a contact group to the backup and updates the count.
func (b *BackupFile) AddGroup(group *people.ContactGroup) {
	b.Groups = append(b.Groups, group)
	b.GroupCount = len(b.Groups)
}

// SaveToFile writes the backup to a JSON file.
func (b *BackupFile) SaveToFile(path string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup data: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// LoadBackupFile loads a backup from a JSON file.
func LoadBackupFile(path string) (*BackupFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}

	var backup BackupFile
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("failed to parse backup file: %w", err)
	}

	// Validate version
	if backup.Version == "" {
		return nil, fmt.Errorf("invalid backup file: missing version")
	}

	return &backup, nil
}

// GetUserGroups returns only user-created contact groups (excludes system groups).
func (b *BackupFile) GetUserGroups() []*people.ContactGroup {
	userGroups := make([]*people.ContactGroup, 0)
	for _, group := range b.Groups {
		if group.GroupType == "USER_CONTACT_GROUP" {
			userGroups = append(userGroups, group)
		}
	}
	return userGroups
}
