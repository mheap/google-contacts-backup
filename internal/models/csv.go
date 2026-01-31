package models

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/people/v1"
)

// CSV column name constants
const (
	colNamePrefix         = "Name Prefix"
	colFirstName          = "First Name"
	colMiddleName         = "Middle Name"
	colLastName           = "Last Name"
	colNameSuffix         = "Name Suffix"
	colPhoneticFirstName  = "Phonetic First Name"
	colPhoneticMiddleName = "Phonetic Middle Name"
	colPhoneticLastName   = "Phonetic Last Name"
	colNickname           = "Nickname"
	colFileAs             = "File As"
	colBirthday           = "Birthday"
	colNotes              = "Notes"
	colLabels             = "Labels"
	colOrgName            = "Organization Name"
	colOrgTitle           = "Organization Title"
	colOrgDepartment      = "Organization Department"
)

// labelSeparator is the separator used between labels in the Labels column
const labelSeparator = " ::: "

// csvFieldCounts tracks the maximum number of each multi-value field across all contacts
type csvFieldCounts struct {
	Emails       int
	Phones       int
	Addresses    int
	Events       int
	Relations    int
	Websites     int
	CustomFields int
}

// countMaxFields determines the maximum count of each multi-value field
func countMaxFields(contacts []*people.Person) csvFieldCounts {
	counts := csvFieldCounts{}

	for _, contact := range contacts {
		if len(contact.EmailAddresses) > counts.Emails {
			counts.Emails = len(contact.EmailAddresses)
		}
		if len(contact.PhoneNumbers) > counts.Phones {
			counts.Phones = len(contact.PhoneNumbers)
		}
		if len(contact.Addresses) > counts.Addresses {
			counts.Addresses = len(contact.Addresses)
		}
		if len(contact.Events) > counts.Events {
			counts.Events = len(contact.Events)
		}
		if len(contact.Relations) > counts.Relations {
			counts.Relations = len(contact.Relations)
		}
		if len(contact.Urls) > counts.Websites {
			counts.Websites = len(contact.Urls)
		}
		if len(contact.UserDefined) > counts.CustomFields {
			counts.CustomFields = len(contact.UserDefined)
		}
	}

	return counts
}

// buildCSVHeaders creates the header row based on field counts
func buildCSVHeaders(counts csvFieldCounts) []string {
	headers := []string{
		colNamePrefix,
		colFirstName,
		colMiddleName,
		colLastName,
		colNameSuffix,
		colPhoneticFirstName,
		colPhoneticMiddleName,
		colPhoneticLastName,
		colNickname,
		colFileAs,
		colBirthday,
		colOrgName,
		colOrgTitle,
		colOrgDepartment,
	}

	// Add email columns
	for i := 1; i <= counts.Emails; i++ {
		headers = append(headers, fmt.Sprintf("Email %d - Label", i))
		headers = append(headers, fmt.Sprintf("Email %d - Value", i))
	}

	// Add phone columns
	for i := 1; i <= counts.Phones; i++ {
		headers = append(headers, fmt.Sprintf("Phone %d - Label", i))
		headers = append(headers, fmt.Sprintf("Phone %d - Value", i))
	}

	// Add address columns
	for i := 1; i <= counts.Addresses; i++ {
		headers = append(headers, fmt.Sprintf("Address %d - Label", i))
		headers = append(headers, fmt.Sprintf("Address %d - Street", i))
		headers = append(headers, fmt.Sprintf("Address %d - Extended Address", i))
		headers = append(headers, fmt.Sprintf("Address %d - City", i))
		headers = append(headers, fmt.Sprintf("Address %d - Region", i))
		headers = append(headers, fmt.Sprintf("Address %d - Postal Code", i))
		headers = append(headers, fmt.Sprintf("Address %d - Country", i))
		headers = append(headers, fmt.Sprintf("Address %d - PO Box", i))
	}

	// Add event columns
	for i := 1; i <= counts.Events; i++ {
		headers = append(headers, fmt.Sprintf("Event %d - Label", i))
		headers = append(headers, fmt.Sprintf("Event %d - Value", i))
	}

	// Add relation columns
	for i := 1; i <= counts.Relations; i++ {
		headers = append(headers, fmt.Sprintf("Relation %d - Label", i))
		headers = append(headers, fmt.Sprintf("Relation %d - Value", i))
	}

	// Add website columns
	for i := 1; i <= counts.Websites; i++ {
		headers = append(headers, fmt.Sprintf("Website %d - Label", i))
		headers = append(headers, fmt.Sprintf("Website %d - Value", i))
	}

	// Add custom field columns
	for i := 1; i <= counts.CustomFields; i++ {
		headers = append(headers, fmt.Sprintf("Custom Field %d - Label", i))
		headers = append(headers, fmt.Sprintf("Custom Field %d - Value", i))
	}

	// Add notes and labels at the end
	headers = append(headers, colNotes, colLabels)

	return headers
}

// contactToCSVRow converts a contact to a CSV row
func contactToCSVRow(contact *people.Person, counts csvFieldCounts, groupNameMap map[string]string) []string {
	row := make([]string, 0)

	// Name fields
	var namePrefix, firstName, middleName, lastName, nameSuffix string
	var phoneticFirst, phoneticMiddle, phoneticLast string
	if len(contact.Names) > 0 {
		name := contact.Names[0]
		namePrefix = name.HonorificPrefix
		firstName = name.GivenName
		middleName = name.MiddleName
		lastName = name.FamilyName
		nameSuffix = name.HonorificSuffix
		phoneticFirst = name.PhoneticGivenName
		phoneticMiddle = name.PhoneticMiddleName
		phoneticLast = name.PhoneticFamilyName
	}

	// Nickname
	var nickname string
	if len(contact.Nicknames) > 0 {
		nickname = contact.Nicknames[0].Value
	}

	// File As
	var fileAs string
	if len(contact.FileAses) > 0 {
		fileAs = contact.FileAses[0].Value
	}

	// Birthday
	var birthday string
	if len(contact.Birthdays) > 0 {
		bday := contact.Birthdays[0]
		if bday.Date != nil {
			if bday.Date.Year > 0 {
				birthday = fmt.Sprintf("%04d-%02d-%02d", bday.Date.Year, bday.Date.Month, bday.Date.Day)
			} else {
				birthday = fmt.Sprintf("--%02d-%02d", bday.Date.Month, bday.Date.Day)
			}
		}
	}

	// Organization
	var orgName, orgTitle, orgDepartment string
	if len(contact.Organizations) > 0 {
		org := contact.Organizations[0]
		orgName = org.Name
		orgTitle = org.Title
		orgDepartment = org.Department
	}

	// Add base fields
	row = append(row,
		namePrefix,
		firstName,
		middleName,
		lastName,
		nameSuffix,
		phoneticFirst,
		phoneticMiddle,
		phoneticLast,
		nickname,
		fileAs,
		birthday,
		orgName,
		orgTitle,
		orgDepartment,
	)

	// Add emails
	for i := 0; i < counts.Emails; i++ {
		if i < len(contact.EmailAddresses) {
			email := contact.EmailAddresses[i]
			row = append(row, normalizeLabel(email.Type), email.Value)
		} else {
			row = append(row, "", "")
		}
	}

	// Add phones
	for i := 0; i < counts.Phones; i++ {
		if i < len(contact.PhoneNumbers) {
			phone := contact.PhoneNumbers[i]
			row = append(row, normalizeLabel(phone.Type), phone.Value)
		} else {
			row = append(row, "", "")
		}
	}

	// Add addresses
	for i := 0; i < counts.Addresses; i++ {
		if i < len(contact.Addresses) {
			addr := contact.Addresses[i]
			row = append(row,
				normalizeLabel(addr.Type),
				addr.StreetAddress,
				addr.ExtendedAddress,
				addr.City,
				addr.Region,
				addr.PostalCode,
				addr.Country,
				addr.PoBox,
			)
		} else {
			row = append(row, "", "", "", "", "", "", "", "")
		}
	}

	// Add events
	for i := 0; i < counts.Events; i++ {
		if i < len(contact.Events) {
			event := contact.Events[i]
			var eventDate string
			if event.Date != nil {
				if event.Date.Year > 0 {
					eventDate = fmt.Sprintf("%04d-%02d-%02d", event.Date.Year, event.Date.Month, event.Date.Day)
				} else {
					eventDate = fmt.Sprintf("--%02d-%02d", event.Date.Month, event.Date.Day)
				}
			}
			row = append(row, normalizeLabel(event.Type), eventDate)
		} else {
			row = append(row, "", "")
		}
	}

	// Add relations
	for i := 0; i < counts.Relations; i++ {
		if i < len(contact.Relations) {
			rel := contact.Relations[i]
			row = append(row, normalizeLabel(rel.Type), rel.Person)
		} else {
			row = append(row, "", "")
		}
	}

	// Add websites
	for i := 0; i < counts.Websites; i++ {
		if i < len(contact.Urls) {
			url := contact.Urls[i]
			row = append(row, normalizeLabel(url.Type), url.Value)
		} else {
			row = append(row, "", "")
		}
	}

	// Add custom fields
	for i := 0; i < counts.CustomFields; i++ {
		if i < len(contact.UserDefined) {
			ud := contact.UserDefined[i]
			row = append(row, ud.Key, ud.Value)
		} else {
			row = append(row, "", "")
		}
	}

	// Notes
	var notes string
	if len(contact.Biographies) > 0 {
		notes = contact.Biographies[0].Value
	}
	row = append(row, notes)

	// Labels (from memberships)
	labels := extractLabels(contact, groupNameMap)
	row = append(row, strings.Join(labels, labelSeparator))

	return row
}

// normalizeLabel converts API type values to user-friendly labels
func normalizeLabel(label string) string {
	if label == "" {
		return ""
	}

	// Remove common prefixes and convert to title case
	label = strings.TrimPrefix(label, "TYPE_")
	label = strings.ToLower(label)

	// Capitalize first letter
	if len(label) > 0 {
		label = strings.ToUpper(label[:1]) + label[1:]
	}

	// Handle specific mappings
	switch strings.ToLower(label) {
	case "home":
		return "Home"
	case "work":
		return "Work"
	case "mobile":
		return "Mobile"
	case "main":
		return "Main"
	case "other":
		return "Other"
	case "homefax":
		return "Home Fax"
	case "workfax":
		return "Work Fax"
	case "pager":
		return "Pager"
	default:
		return label
	}
}

// extractLabels extracts user-created group labels from contact memberships
func extractLabels(contact *people.Person, groupNameMap map[string]string) []string {
	labels := make([]string, 0)

	for _, membership := range contact.Memberships {
		if membership.ContactGroupMembership != nil {
			resourceName := membership.ContactGroupMembership.ContactGroupResourceName

			// Skip system groups
			if isSystemGroup(resourceName) {
				continue
			}

			// Look up group name
			if name, ok := groupNameMap[resourceName]; ok {
				labels = append(labels, name)
			}
		}
	}

	return labels
}

// isSystemGroup returns true if the resource name is a system group
func isSystemGroup(resourceName string) bool {
	systemGroups := []string{
		"contactGroups/myContacts",
		"contactGroups/starred",
		"contactGroups/chatBuddies",
		"contactGroups/all",
		"contactGroups/friends",
		"contactGroups/family",
		"contactGroups/coworkers",
		"contactGroups/blocked",
	}

	for _, sg := range systemGroups {
		if resourceName == sg {
			return true
		}
	}

	return false
}

// SaveToCSV writes the backup to a Google-compatible CSV file.
func (b *BackupFile) SaveToCSV(path string) error {
	// Build group name lookup map
	groupNameMap := make(map[string]string)
	for _, group := range b.Groups {
		if group.GroupType == "USER_CONTACT_GROUP" {
			groupNameMap[group.ResourceName] = group.Name
		}
	}

	// Count max fields
	counts := countMaxFields(b.Contacts)

	// Ensure at least one of each multi-value field for consistent output
	if counts.Emails == 0 {
		counts.Emails = 1
	}
	if counts.Phones == 0 {
		counts.Phones = 1
	}

	// Build headers
	headers := buildCSVHeaders(counts)

	// Create file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write contacts
	for _, contact := range b.Contacts {
		row := contactToCSVRow(contact, counts, groupNameMap)
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}
