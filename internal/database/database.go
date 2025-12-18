package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps SQLite database
type DB struct {
	conn *sql.DB
}

// VolumeMetadata represents volume metadata
type VolumeMetadata struct {
	VolumeName   string
	LastAccessed time.Time
	LastBackup   time.Time
	BackupCount  int
}

// BackupRecord represents a backup record
type BackupRecord struct {
	ID           int
	VolumeName   string
	ServiceName  string
	ProjectName  string
	FilePath     string
	Size         int64
	CreatedAt    time.Time
	Tag          string
	Checksum     string
}

// NewDB creates a new database connection
func NewDB(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Configure connection pool settings suitable for SQLite
	// SQLite typically benefits from a very small number of open connections
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	// Configure recommended SQLite pragmas for better concurrency and data integrity
	if _, err := conn.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		conn.Close()
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.initialize(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// initialize creates the necessary tables
func (db *DB) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS volume_metadata (
		volume_name TEXT PRIMARY KEY,
		last_accessed TIMESTAMP,
		last_backup TIMESTAMP,
		backup_count INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS backup_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		volume_name TEXT NOT NULL,
		service_name TEXT,
		project_name TEXT,
		file_path TEXT NOT NULL,
		size INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		tag TEXT,
		checksum TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_volume_name ON backup_records(volume_name);
	CREATE INDEX IF NOT EXISTS idx_project_name ON backup_records(project_name);
	CREATE INDEX IF NOT EXISTS idx_created_at ON backup_records(created_at);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// UpdateLastAccessed updates the last accessed time for a volume
func (db *DB) UpdateLastAccessed(volumeName string) error {
	query := `
	INSERT INTO volume_metadata (volume_name, last_accessed, backup_count)
	VALUES (?, ?, 0)
	ON CONFLICT(volume_name) DO UPDATE SET last_accessed = ?
	`
	now := time.Now()
	_, err := db.conn.Exec(query, volumeName, now, now)
	return err
}

// UpdateLastBackup updates the last backup time for a volume
func (db *DB) UpdateLastBackup(volumeName string) error {
	query := `
	INSERT INTO volume_metadata (volume_name, last_backup, backup_count)
	VALUES (?, ?, 1)
	ON CONFLICT(volume_name) DO UPDATE SET
		last_backup = ?,
		backup_count = backup_count + 1
	`
	now := time.Now()
	_, err := db.conn.Exec(query, volumeName, now, now)
	return err
}

// GetVolumeMetadata gets metadata for a volume
func (db *DB) GetVolumeMetadata(volumeName string) (*VolumeMetadata, error) {
	query := `
	SELECT volume_name, last_accessed, last_backup, backup_count
	FROM volume_metadata
	WHERE volume_name = ?
	`

	var meta VolumeMetadata
	var lastAccessed, lastBackup sql.NullTime

	err := db.conn.QueryRow(query, volumeName).Scan(
		&meta.VolumeName,
		&lastAccessed,
		&lastBackup,
		&meta.BackupCount,
	)

	if err == sql.ErrNoRows {
		return &VolumeMetadata{
			VolumeName: volumeName,
		}, nil
	}

	if err != nil {
		return nil, err
	}

	if lastAccessed.Valid {
		meta.LastAccessed = lastAccessed.Time
	}
	if lastBackup.Valid {
		meta.LastBackup = lastBackup.Time
	}

	return &meta, nil
}

// AddBackupRecord adds a backup record
func (db *DB) AddBackupRecord(record *BackupRecord) error {
	query := `
	INSERT INTO backup_records (volume_name, service_name, project_name, file_path, size, tag, checksum)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.conn.Exec(query,
		record.VolumeName,
		record.ServiceName,
		record.ProjectName,
		record.FilePath,
		record.Size,
		record.Tag,
		record.Checksum,
	)

	return err
}

// GetBackupRecords gets backup records for a volume
func (db *DB) GetBackupRecords(volumeName string, limit int) ([]*BackupRecord, error) {
	query := `
	SELECT id, volume_name, service_name, project_name, file_path, size, created_at, tag, checksum
	FROM backup_records
	WHERE volume_name = ?
	ORDER BY created_at DESC
	`

	if limit > 0 {
		query += " LIMIT ?"
	}

	var rows *sql.Rows
	var err error

	if limit > 0 {
		rows, err = db.conn.Query(query, volumeName, limit)
	} else {
		rows, err = db.conn.Query(query, volumeName)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*BackupRecord
	for rows.Next() {
		var record BackupRecord
		var serviceName, projectName, tag, checksum sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.VolumeName,
			&serviceName,
			&projectName,
			&record.FilePath,
			&record.Size,
			&record.CreatedAt,
			&tag,
			&checksum,
		)
		if err != nil {
			return nil, err
		}

		if serviceName.Valid {
			record.ServiceName = serviceName.String
		}
		if projectName.Valid {
			record.ProjectName = projectName.String
		}
		if tag.Valid {
			record.Tag = tag.String
		}
		if checksum.Valid {
			record.Checksum = checksum.String
		}

		records = append(records, &record)
	}

	return records, rows.Err()
}

// GetAllBackupRecords gets all backup records
func (db *DB) GetAllBackupRecords(limit int) ([]*BackupRecord, error) {
	query := `
	SELECT id, volume_name, service_name, project_name, file_path, size, created_at, tag, checksum
	FROM backup_records
	ORDER BY created_at DESC
	`

	if limit > 0 {
		query += " LIMIT ?"
	}

	var rows *sql.Rows
	var err error

	if limit > 0 {
		rows, err = db.conn.Query(query, limit)
	} else {
		rows, err = db.conn.Query(query)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*BackupRecord
	for rows.Next() {
		var record BackupRecord
		var serviceName, projectName, tag, checksum sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.VolumeName,
			&serviceName,
			&projectName,
			&record.FilePath,
			&record.Size,
			&record.CreatedAt,
			&tag,
			&checksum,
		)
		if err != nil {
			return nil, err
		}

		if serviceName.Valid {
			record.ServiceName = serviceName.String
		}
		if projectName.Valid {
			record.ProjectName = projectName.String
		}
		if tag.Valid {
			record.Tag = tag.String
		}
		if checksum.Valid {
			record.Checksum = checksum.String
		}

		records = append(records, &record)
	}

	return records, rows.Err()
}

// GetStaleVolumes gets volumes not accessed for the specified number of days
func (db *DB) GetStaleVolumes(days int) ([]string, error) {
	query := `
	SELECT volume_name
	FROM volume_metadata
	WHERE last_accessed < datetime('now', '-' || ? || ' days')
	`

	rows, err := db.conn.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var volumes []string
	for rows.Next() {
		var volumeName string
		if err := rows.Scan(&volumeName); err != nil {
			return nil, err
		}
		volumes = append(volumes, volumeName)
	}

	return volumes, rows.Err()
}

// DeleteBackupRecord deletes a backup record
func (db *DB) DeleteBackupRecord(id int) error {
	query := `DELETE FROM backup_records WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	return err
}

// CleanupOldBackups deletes old backup records beyond keep_generations
func (db *DB) CleanupOldBackups(volumeName string, keepGenerations int) ([]*BackupRecord, error) {
	// Get all records for this volume
	records, err := db.GetBackupRecords(volumeName, 0)
	if err != nil {
		return nil, err
	}

	// If we have more than keepGenerations, delete the oldest
	if len(records) > keepGenerations {
		toDelete := records[keepGenerations:]
		for _, record := range toDelete {
			if err := db.DeleteBackupRecord(record.ID); err != nil {
				return nil, err
			}
		}
		return toDelete, nil
	}

	return nil, nil
}
