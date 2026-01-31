// Package contacts provides a client for Google People API operations.
package contacts

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

const (
	// personFields is the list of fields to request for each contact
	personFields = "names,emailAddresses,phoneNumbers,addresses,organizations,birthdays,biographies,urls,photos,userDefined,events,relations,memberships,nicknames,occupations,genders,imClients,interests,sipAddresses,calendarUrls,externalIds,locales,locations,miscKeywords,clientData"

	// maxPageSize is the maximum number of contacts per page
	maxPageSize = 1000

	// batchDeleteSize is the maximum number of contacts to delete in one batch
	batchDeleteSize = 500

	// batchCreateSize is the maximum number of contacts to create in one batch
	batchCreateSize = 200

	// rateLimitDelay is the delay between API calls to avoid rate limiting
	rateLimitDelay = 100 * time.Millisecond
)

// Client wraps the Google People API service.
type Client struct {
	service *people.Service
}

// NewClient creates a new People API client.
func NewClient(ctx context.Context, httpClient *http.Client) (*Client, error) {
	service, err := people.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create People API service: %w", err)
	}

	return &Client{service: service}, nil
}

// ListContacts retrieves all contacts with pagination.
// The progressFn callback is called with (current, total) after each page.
func (c *Client) ListContacts(ctx context.Context, progressFn func(current, total int)) ([]*people.Person, error) {
	var allContacts []*people.Person
	var pageToken string
	totalCount := 0

	for {
		call := c.service.People.Connections.List("people/me").
			PersonFields(personFields).
			PageSize(maxPageSize).
			Context(ctx)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list contacts: %w", err)
		}

		// Update total count from first response
		if totalCount == 0 && resp.TotalPeople > 0 {
			totalCount = int(resp.TotalPeople)
		}

		allContacts = append(allContacts, resp.Connections...)

		if progressFn != nil {
			progressFn(len(allContacts), totalCount)
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}

		time.Sleep(rateLimitDelay)
	}

	return allContacts, nil
}

// ListGroups retrieves all contact groups.
func (c *Client) ListGroups(ctx context.Context) ([]*people.ContactGroup, error) {
	var allGroups []*people.ContactGroup
	var pageToken string

	for {
		call := c.service.ContactGroups.List().
			PageSize(1000).
			Context(ctx)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list contact groups: %w", err)
		}

		allGroups = append(allGroups, resp.ContactGroups...)

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}

		time.Sleep(rateLimitDelay)
	}

	return allGroups, nil
}

// DeleteAllContacts deletes all contacts in batches.
// The progressFn callback is called with (deleted, total) after each batch.
func (c *Client) DeleteAllContacts(ctx context.Context, progressFn func(deleted, total int)) error {
	// First, get all contact resource names
	contacts, err := c.ListContacts(ctx, nil)
	if err != nil {
		return err
	}

	if len(contacts) == 0 {
		return nil
	}

	totalContacts := len(contacts)

	// Extract resource names
	resourceNames := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		if contact.ResourceName != "" {
			resourceNames = append(resourceNames, contact.ResourceName)
		}
	}

	// Delete in batches
	deleted := 0
	for i := 0; i < len(resourceNames); i += batchDeleteSize {
		end := i + batchDeleteSize
		if end > len(resourceNames) {
			end = len(resourceNames)
		}

		batch := resourceNames[i:end]

		req := &people.BatchDeleteContactsRequest{
			ResourceNames: batch,
		}

		_, err := c.service.People.BatchDeleteContacts(req).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to delete contacts batch: %w", err)
		}

		deleted += len(batch)
		if progressFn != nil {
			progressFn(deleted, totalContacts)
		}

		time.Sleep(rateLimitDelay)
	}

	return nil
}

// DeleteUserGroups deletes all user-created contact groups.
// The progressFn callback is called with (deleted, total) after each deletion.
func (c *Client) DeleteUserGroups(ctx context.Context, progressFn func(deleted, total int)) error {
	groups, err := c.ListGroups(ctx)
	if err != nil {
		return err
	}

	// Filter to only user-created groups
	var userGroups []*people.ContactGroup
	for _, group := range groups {
		if group.GroupType == "USER_CONTACT_GROUP" {
			userGroups = append(userGroups, group)
		}
	}

	if len(userGroups) == 0 {
		return nil
	}

	totalGroups := len(userGroups)
	deleted := 0

	for _, group := range userGroups {
		_, err := c.service.ContactGroups.Delete(group.ResourceName).
			DeleteContacts(false). // Don't delete contacts, just the group
			Context(ctx).
			Do()

		if err != nil {
			// Log warning but continue with other groups
			fmt.Printf("Warning: failed to delete group %s: %v\n", group.Name, err)
		} else {
			deleted++
		}

		if progressFn != nil {
			progressFn(deleted, totalGroups)
		}

		time.Sleep(rateLimitDelay)
	}

	return nil
}

// CreateGroups creates contact groups from the backup.
// Returns a map of old resource names to new resource names.
func (c *Client) CreateGroups(ctx context.Context, groups []*people.ContactGroup, progressFn func(created, total int)) (map[string]string, error) {
	resourceNameMap := make(map[string]string)
	totalGroups := len(groups)
	created := 0

	for _, group := range groups {
		// Only create user contact groups
		if group.GroupType != "USER_CONTACT_GROUP" {
			continue
		}

		req := &people.CreateContactGroupRequest{
			ContactGroup: &people.ContactGroup{
				Name: group.Name,
			},
		}

		newGroup, err := c.service.ContactGroups.Create(req).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to create group %s: %w", group.Name, err)
		}

		// Map old resource name to new one
		resourceNameMap[group.ResourceName] = newGroup.ResourceName
		created++

		if progressFn != nil {
			progressFn(created, totalGroups)
		}

		time.Sleep(rateLimitDelay)
	}

	return resourceNameMap, nil
}

// CreateContacts creates contacts from the backup in batches.
// groupMap maps old group resource names to new ones for updating memberships.
func (c *Client) CreateContacts(ctx context.Context, contacts []*people.Person, groupMap map[string]string, progressFn func(created, total int)) error {
	if len(contacts) == 0 {
		return nil
	}

	totalContacts := len(contacts)
	created := 0

	// Process in batches
	for i := 0; i < len(contacts); i += batchCreateSize {
		end := i + batchCreateSize
		if end > len(contacts) {
			end = len(contacts)
		}

		batch := contacts[i:end]

		// Prepare contacts for creation
		contactsToCreate := make([]*people.ContactToCreate, 0, len(batch))
		for _, contact := range batch {
			// Clean the contact for creation (remove server-assigned fields)
			cleanContact := cleanContactForCreation(contact, groupMap)
			contactsToCreate = append(contactsToCreate, &people.ContactToCreate{
				ContactPerson: cleanContact,
			})
		}

		req := &people.BatchCreateContactsRequest{
			Contacts: contactsToCreate,
			ReadMask: "names",
			Sources:  []string{"READ_SOURCE_TYPE_CONTACT"},
		}

		_, err := c.service.People.BatchCreateContacts(req).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to create contacts batch: %w", err)
		}

		created += len(batch)
		if progressFn != nil {
			progressFn(created, totalContacts)
		}

		time.Sleep(rateLimitDelay)
	}

	return nil
}

// cleanContactForCreation removes server-assigned fields and updates group memberships.
func cleanContactForCreation(contact *people.Person, groupMap map[string]string) *people.Person {
	// Create a new person with only the fields we can set
	newPerson := &people.Person{
		Names:          contact.Names,
		Nicknames:      contact.Nicknames,
		EmailAddresses: contact.EmailAddresses,
		PhoneNumbers:   contact.PhoneNumbers,
		Addresses:      contact.Addresses,
		Organizations:  contact.Organizations,
		Birthdays:      contact.Birthdays,
		Biographies:    contact.Biographies,
		Urls:           contact.Urls,
		UserDefined:    contact.UserDefined,
		Events:         contact.Events,
		Relations:      contact.Relations,
		Occupations:    contact.Occupations,
		Genders:        contact.Genders,
		ImClients:      contact.ImClients,
		Interests:      contact.Interests,
		SipAddresses:   contact.SipAddresses,
		CalendarUrls:   contact.CalendarUrls,
		ExternalIds:    contact.ExternalIds,
		Locales:        contact.Locales,
		Locations:      contact.Locations,
		MiscKeywords:   contact.MiscKeywords,
		ClientData:     contact.ClientData,
	}

	// Update memberships with new group resource names
	if len(contact.Memberships) > 0 {
		newMemberships := make([]*people.Membership, 0)
		for _, membership := range contact.Memberships {
			if membership.ContactGroupMembership != nil {
				oldResourceName := membership.ContactGroupMembership.ContactGroupResourceName

				// Check if it's a system group (myContacts, starred, etc.)
				if strings.HasPrefix(oldResourceName, "contactGroups/") &&
					!strings.HasPrefix(oldResourceName, "contactGroups/myContacts") &&
					!strings.HasPrefix(oldResourceName, "contactGroups/starred") &&
					!strings.HasPrefix(oldResourceName, "contactGroups/chatBuddies") &&
					!strings.HasPrefix(oldResourceName, "contactGroups/all") &&
					!strings.HasPrefix(oldResourceName, "contactGroups/friends") &&
					!strings.HasPrefix(oldResourceName, "contactGroups/family") &&
					!strings.HasPrefix(oldResourceName, "contactGroups/coworkers") {
					// This is a user group, map to new resource name
					if newResourceName, ok := groupMap[oldResourceName]; ok {
						newMemberships = append(newMemberships, &people.Membership{
							ContactGroupMembership: &people.ContactGroupMembership{
								ContactGroupResourceName: newResourceName,
							},
						})
					}
				} else if oldResourceName == "contactGroups/myContacts" {
					// Keep myContacts membership
					newMemberships = append(newMemberships, membership)
				}
			}
		}
		newPerson.Memberships = newMemberships
	}

	// Clear metadata from nested objects
	clearFieldMetadata(newPerson)

	return newPerson
}

// clearFieldMetadata removes server-assigned metadata from all fields.
func clearFieldMetadata(person *people.Person) {
	for _, name := range person.Names {
		name.Metadata = nil
	}
	for _, nickname := range person.Nicknames {
		nickname.Metadata = nil
	}
	for _, email := range person.EmailAddresses {
		email.Metadata = nil
	}
	for _, phone := range person.PhoneNumbers {
		phone.Metadata = nil
	}
	for _, addr := range person.Addresses {
		addr.Metadata = nil
	}
	for _, org := range person.Organizations {
		org.Metadata = nil
	}
	for _, bday := range person.Birthdays {
		bday.Metadata = nil
	}
	for _, bio := range person.Biographies {
		bio.Metadata = nil
	}
	for _, url := range person.Urls {
		url.Metadata = nil
	}
	for _, ud := range person.UserDefined {
		ud.Metadata = nil
	}
	for _, event := range person.Events {
		event.Metadata = nil
	}
	for _, rel := range person.Relations {
		rel.Metadata = nil
	}
	for _, occ := range person.Occupations {
		occ.Metadata = nil
	}
	for _, gender := range person.Genders {
		gender.Metadata = nil
	}
	for _, im := range person.ImClients {
		im.Metadata = nil
	}
	for _, interest := range person.Interests {
		interest.Metadata = nil
	}
	for _, sip := range person.SipAddresses {
		sip.Metadata = nil
	}
	for _, cal := range person.CalendarUrls {
		cal.Metadata = nil
	}
	for _, ext := range person.ExternalIds {
		ext.Metadata = nil
	}
	for _, locale := range person.Locales {
		locale.Metadata = nil
	}
	for _, loc := range person.Locations {
		loc.Metadata = nil
	}
	for _, misc := range person.MiscKeywords {
		misc.Metadata = nil
	}
	for _, cd := range person.ClientData {
		cd.Metadata = nil
	}
}
