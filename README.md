# Google Contacts Backup

A CLI tool to backup and restore your Google Contacts to/from JSON or CSV files.

## Features

- **Full Backup**: Downloads all contact fields including names, emails, phones, addresses, organizations, birthdays, notes, custom fields, and more
- **Multiple Formats**: Export as JSON (full backup with restore support) or Google-compatible CSV
- **Contact Groups**: Backs up and restores contact groups (labels)
- **OAuth2 Authentication**: Secure browser-based authentication with token caching
- **Progress Indicators**: Visual progress bars for all operations
- **Safe Restore**: Confirmation prompt before destructive restore operations

## Installation

### From Source

```bash
go install github.com/mheap/google-contacts-backup@latest
```

Or clone and build:

```bash
git clone https://github.com/mheap/google-contacts-backup.git
cd google-contacts-backup
go build -o google-contacts-backup .
```

## Setup

Before using this tool, you need to set up Google Cloud credentials:

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (or select an existing one)
3. Enable the **People API**:
   - Navigate to "APIs & Services" > "Library"
   - Search for "People API" and enable it
4. Create OAuth 2.0 credentials:
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth client ID"
   - Select "Desktop app" as the application type
   - Give it a name and click "Create"
5. Download the credentials JSON file
6. Save it to the config directory:
   - Linux/macOS: `$XDG_CONFIG_HOME/google-contacts-backup/credentials.json` (defaults to `~/.config/google-contacts-backup/credentials.json`)
   - Or specify a custom path with `--credentials`

## Usage

### Authenticate

Before backing up or restoring contacts, you need to authenticate with Google:

```bash
# Authenticate with Google (opens browser)
google-contacts-backup auth

# Use a custom credentials file
google-contacts-backup auth -c ~/path/to/credentials.json
```

This will:
1. Start a local server to receive the OAuth callback
2. Open your browser to Google's consent page
3. Save your tokens locally for future use

You only need to run this once. The tokens are cached and automatically refreshed.

### Backup Contacts

The backup command supports two output formats:
- **JSON** (default): Full backup that can be restored using this tool
- **CSV**: Google-compatible format that can be imported via the Google Contacts web UI

```bash
# Backup to a timestamped JSON file (default)
google-contacts-backup backup

# Backup to a specific JSON file
google-contacts-backup backup -o my-contacts.json

# Backup as Google-compatible CSV
google-contacts-backup backup --format csv
google-contacts-backup backup -f csv -o my-contacts.csv

# Use a custom credentials file
google-contacts-backup backup -c ~/path/to/credentials.json -o backup.json
```

### Restore Contacts

> **Warning**: The restore operation is **destructive**! It will delete ALL existing contacts and contact groups before restoring from the backup file. Always create a fresh backup before restoring.

```bash
# Restore from a backup file (will prompt for confirmation)
google-contacts-backup restore -i my-contacts.json

# Restore without confirmation prompt (for scripting)
google-contacts-backup restore -i my-contacts.json --confirm

# Create a safety backup before restoring
google-contacts-backup backup -o pre-restore-backup.json
google-contacts-backup restore -i old-backup.json
```

### Global Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--credentials` | `-c` | Path to OAuth credentials JSON file | `$XDG_CONFIG_HOME/google-contacts-backup/credentials.json` |
| `--help` | `-h` | Show help | |
| `--version` | `-v` | Show version | |

### Auth Command Options

The `auth` command has no additional options beyond the global `--credentials` flag.

### Backup Command Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--output` | `-o` | Output file path | `contacts-YYYYMMDD-HHMMSS.json` (or `.csv`) |
| `--format` | `-f` | Output format: `json` or `csv` | `json` |

### Restore Command Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--input` | `-i` | Input backup file path (required) | |
| `--confirm` | | Skip confirmation prompt | `false` |

## Backup File Formats

### JSON Format

The JSON format provides a complete backup that can be restored using this tool:

```json
{
  "version": "1.0",
  "created_at": "2024-01-15T10:30:00Z",
  "contact_count": 150,
  "group_count": 5,
  "contacts": [
    {
      "resourceName": "people/c123456789",
      "names": [{"givenName": "John", "familyName": "Doe"}],
      "emailAddresses": [{"value": "john@example.com"}],
      ...
    }
  ],
  "groups": [
    {
      "resourceName": "contactGroups/abc123",
      "name": "Work",
      "groupType": "USER_CONTACT_GROUP",
      ...
    }
  ]
}
```

### CSV Format

The CSV format is compatible with Google Contacts import. It uses the official Google CSV format with columns like:

- Name fields: `Name Prefix`, `First Name`, `Middle Name`, `Last Name`, `Name Suffix`, `Nickname`
- Contact fields: `Email 1 - Label`, `Email 1 - Value`, `Phone 1 - Label`, `Phone 1 - Value`, etc.
- Address fields: `Address 1 - Street`, `Address 1 - City`, `Address 1 - Region`, etc.
- Organization: `Organization Name`, `Organization Title`, `Organization Department`
- Other: `Birthday`, `Notes`, `Labels`

To import a CSV backup:
1. Go to [Google Contacts](https://contacts.google.com)
2. Click "Import" in the left sidebar
3. Select your CSV file and click "Import"

## Limitations

- **Contact Photos**: Photos are stored as URLs in JSON backups, but they cannot be restored via the Google People API. The URLs may also expire over time. Photos are not included in CSV exports.
- **CSV Restore**: CSV files can only be imported via the Google Contacts web UI, not restored using this tool. Use JSON format for full backup/restore capability.
- **System Groups**: System contact groups (My Contacts, Starred, etc.) cannot be deleted or recreated. Only user-created groups are backed up and restored.
- **Read-Only Fields**: Some server-assigned fields (like `resourceName`, `etag`, and metadata) are stripped during restore as new contacts receive new identifiers.

## Authentication Flow

You can authenticate explicitly using the `auth` command, or authentication will happen automatically when you run `backup` or `restore` for the first time.

The authentication flow:

1. Start a local HTTP server on a random port
2. Open your default browser to Google's consent page
3. After you authorize, Google redirects back to the local server
4. The tool exchanges the authorization code for access/refresh tokens
5. Tokens are cached in `~/.google-contacts-backup/token.json`

Subsequent runs will use the cached refresh token automatically.

## API Rate Limits

The tool includes built-in rate limiting (100ms delay between API calls) and uses batch operations where possible to stay within Google's API quotas:

- Batch delete: up to 500 contacts per request
- Batch create: up to 200 contacts per request

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
