package types

import "errors"

// Sentinel errors for common failure conditions across packages.
var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists indicates a resource with the same identity already exists.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalidConfig indicates the configuration is malformed or missing required fields.
	ErrInvalidConfig = errors.New("invalid config")
	// ErrGateFailed indicates a verification gate did not pass.
	ErrGateFailed = errors.New("gate failed")
	// ErrPermissionDenied indicates the operation is not allowed under the agent's permission tier.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrRateLimited indicates the agent has exceeded its rate limit.
	ErrRateLimited = errors.New("rate limited")
)
