# Docker Volume Manager (dvm)

A powerful CLI tool for managing Docker volume lifecycles, built with Go.

## Overview

`dvm` is designed to integrate seamlessly with Docker Compose environments, providing easy operations for backup, restore, archive, swap, and cleanup of Docker volumes.

## Key Features

- **Docker Compose Integration**: Automatic volume detection from Compose files
- **Backup/Restore**: Simple volume data backup and restoration
- **Archive**: Archive and remove unused volumes
- **Swap**: Easily swap volume contents (e.g., switching between test and production data)
- **Cleanup**: Automatic detection and removal of unused volumes
- **History Tracking**: Track backup history
- **Metadata Tracking**: Monitor last access times and usage patterns

## Installation

### Build from Source

```bash
# Clone the repository
git clone https://github.com/koyashimano/docker-volume-manager.git
cd docker-volume-manager

# Build
go build -o dvm ./cmd/dvm

# Install to system (optional)
sudo mv dvm /usr/local/bin/
```

Alternatively, use the Makefile:

```bash
make build          # Build binary
make install        # Install to /usr/local/bin
```

### Requirements

- Go 1.21 or higher
- Docker installed and running

## Quick Start

```bash
# Navigate to your project directory (containing compose.yaml)
cd ~/projects/myapp

# List volumes
dvm list

# Backup all volumes
dvm backup

# Backup specific service volume
dvm backup db

# Restore from latest backup
dvm restore db

# Restore with interactive selection
dvm restore db --select

# Cleanup unused volumes (dry-run)
dvm clean --unused --dry-run

# Actually cleanup
dvm clean --unused
```

## Usage

### Global Options

```
-f, --file <path>      Path to Compose file
-p, --project <name>   Override project name
--no-compose           Disable Compose integration
-v, --verbose          Verbose output
-q, --quiet            Minimal output
--config <path>        Specify config file path
--version              Show version
-h, --help             Show help
```

### Commands

#### `dvm list` - List volumes

```bash
dvm list                    # Current project volumes
dvm list --all             # All volumes
dvm list --unused          # Only unused volumes
dvm list --stale 30        # Not accessed for 30+ days
dvm list --format json     # Output as JSON
```

#### `dvm backup` - Create backups

```bash
dvm backup                  # Backup all volumes
dvm backup db              # Backup db service volume
dvm backup db redis        # Backup multiple services
dvm backup -o /backup      # Specify output directory
dvm backup --tag daily     # Tag the backup
dvm backup --stop          # Stop containers before backup
```

#### `dvm restore` - Restore from backup

```bash
dvm restore db             # Restore from latest backup
dvm restore db --select    # Interactive backup selection
dvm restore db --list      # List available backups
dvm restore --restart      # Restart containers after restore
dvm restore /path/to/backup.tar.gz  # Restore from specific file
```

#### `dvm archive` - Archive and delete

```bash
dvm archive                # Archive entire project
dvm archive db             # Archive specific service only
dvm archive --verify       # Verify integrity before deletion
```

#### `dvm swap` - Swap volumes

```bash
dvm swap db --empty --restart           # Swap to empty volume
dvm swap db test_data.tar.gz --restart  # Swap to test data
dvm restore db --restart                # Restore original
```

#### `dvm clean` - Cleanup volumes

```bash
dvm clean --unused --dry-run    # Preview what will be deleted
dvm clean --unused              # Delete unused volumes
dvm clean --stale 60            # Delete volumes unused for 60+ days
dvm clean --unused --archive    # Archive before deleting
```

#### `dvm history` - Show backup history

```bash
dvm history                # Current project history
dvm history db             # Specific service history
dvm history --all          # All projects
dvm history -n 20          # Show 20 entries
```

#### `dvm inspect` - Show detailed information

```bash
dvm inspect db             # Show volume details
dvm inspect db --format json  # Output as JSON
```

#### `dvm clone` - Clone volumes

```bash
dvm clone db db_test       # Clone for testing
```

## Configuration

Customize settings in `~/.dvm/config.yaml`:

```yaml
# Default settings
defaults:
  compress_format: tar.gz    # tar.gz | tar.zst
  keep_generations: 5        # Number of backup generations to keep
  stop_before_backup: false  # Stop containers before backup

# Path settings
paths:
  backups: ~/.dvm/backups
  archives: ~/.dvm/archives

# Project-specific settings
projects:
  myproject:
    keep_generations: 10
```

## Directory Structure

```
~/.dvm/
├── config.yaml              # Global configuration
├── backups/                 # Backups
│   ├── myproject/
│   │   ├── db_2024-12-18_143022.tar.gz
│   │   └── redis_2024-12-18_143022.tar.gz
│   └── other-project/
├── archives/                # Archived volumes
└── meta.db                  # Metadata (SQLite)
```

## Common Workflows

### Daily Backups

```bash
cd ~/projects/myapp
dvm backup
dvm history
```

### Reset Development Environment

```bash
dvm swap db --empty --restart
# ... development work ...
dvm restore db --restart
```

### Test with Production Data

```bash
dvm swap db ./prod_dump.tar.gz --restart
# ... testing ...
dvm restore db --restart
```

### Project Cleanup

```bash
cd ~/projects/finished-project
dvm archive --verify
```

### Periodic Cleanup

```bash
dvm clean --stale 90 --dry-run
dvm clean --stale 90 --archive
```

## Troubleshooting

### Docker Connection Error

Ensure Docker is running:

```bash
docker ps
```

### Permission Error

Verify you have permission to run Docker commands:

```bash
docker volume ls
```

## License

MIT

## Specification

For detailed specification, see [spec.md](spec.md).
