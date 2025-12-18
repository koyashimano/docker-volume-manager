package commands

import "errors"

var (
	// ErrVolumeNotFound is returned when a volume is not found
	ErrVolumeNotFound = errors.New("volume not found")

	// ErrServiceNotFound is returned when a service is not found
	ErrServiceNotFound = errors.New("service not found")

	// ErrComposeNotFound is returned when compose file is not found
	ErrComposeNotFound = errors.New("compose file not found")

	// ErrVolumeInUse is returned when a volume is in use
	ErrVolumeInUse = errors.New("volume is in use by running containers")

	// ErrBackupNotFound is returned when a backup is not found
	ErrBackupNotFound = errors.New("backup not found")

	// ErrInsufficientSpace is returned when there's not enough disk space
	ErrInsufficientSpace = errors.New("insufficient disk space")
)

// ExitCode represents program exit codes
type ExitCode int

const (
	ExitSuccess ExitCode = 0
	ExitError   ExitCode = 1
	ExitNotFound ExitCode = 2
	ExitPermission ExitCode = 3
	ExitDiskFull ExitCode = 4
	ExitInUse ExitCode = 5
	ExitNoCompose ExitCode = 6
)

// GetExitCode returns the appropriate exit code for an error
func GetExitCode(err error) ExitCode {
	if err == nil {
		return ExitSuccess
	}

	switch err {
	case ErrVolumeNotFound, ErrServiceNotFound, ErrBackupNotFound:
		return ExitNotFound
	case ErrComposeNotFound:
		return ExitNoCompose
	case ErrVolumeInUse:
		return ExitInUse
	case ErrInsufficientSpace:
		return ExitDiskFull
	default:
		return ExitError
	}
}
