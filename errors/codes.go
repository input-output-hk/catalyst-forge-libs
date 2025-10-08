// Package errors provides a foundational error handling system for the Catalyst Forge platform.
// It extends Go's standard error handling with structured error codes, retry classification,
// context preservation, and API serialization capabilities.
package errors

// ErrorCode represents a specific error condition in the Catalyst Forge platform.
// Error codes are string-based for debuggability and natural JSON serialization.
type ErrorCode string

const (
	// Resource errors.

	// CodeNotFound indicates a requested resource does not exist.
	CodeNotFound ErrorCode = "NOT_FOUND"

	// CodeAlreadyExists indicates a resource already exists and cannot be created again.
	CodeAlreadyExists ErrorCode = "ALREADY_EXISTS"

	// CodeConflict indicates a resource state conflict that prevents the operation.
	CodeConflict ErrorCode = "CONFLICT"

	// Permission errors.

	// CodeUnauthorized indicates the request lacks valid authentication credentials.
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"

	// CodeForbidden indicates the authenticated user lacks permission for the operation.
	CodeForbidden ErrorCode = "FORBIDDEN"

	// Validation errors.

	// CodeInvalidInput indicates the provided input is invalid or malformed.
	CodeInvalidInput ErrorCode = "INVALID_INPUT"

	// CodeInvalidConfig indicates a configuration error prevents the operation.
	CodeInvalidConfig ErrorCode = "INVALID_CONFIGURATION"

	// CodeSchemaFailed indicates the data failed schema validation.
	CodeSchemaFailed ErrorCode = "SCHEMA_VALIDATION_FAILED"

	// Infrastructure errors.

	// CodeDatabase indicates a database operation failed.
	CodeDatabase ErrorCode = "DATABASE_ERROR"

	// CodeNetwork indicates a network operation failed.
	CodeNetwork ErrorCode = "NETWORK_ERROR"

	// CodeTimeout indicates an operation exceeded its time limit.
	CodeTimeout ErrorCode = "TIMEOUT"

	// CodeRateLimit indicates the rate limit has been exceeded.
	CodeRateLimit ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Execution errors.

	// CodeExecutionFailed indicates a general execution failure.
	CodeExecutionFailed ErrorCode = "EXECUTION_FAILED"

	// CodeBuildFailed indicates a build operation failed.
	CodeBuildFailed ErrorCode = "BUILD_FAILED"

	// CodePublishFailed indicates a publish operation failed.
	CodePublishFailed ErrorCode = "PUBLISH_FAILED"

	// System errors.

	// CodeInternal indicates an internal system error occurred.
	CodeInternal ErrorCode = "INTERNAL_ERROR"

	// CodeNotImplemented indicates the requested functionality is not implemented.
	CodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"

	// CodeUnavailable indicates the service is temporarily unavailable.
	CodeUnavailable ErrorCode = "SERVICE_UNAVAILABLE"

	// Generic errors.

	// CodeUnknown indicates an unknown or unclassified error occurred.
	CodeUnknown ErrorCode = "UNKNOWN"
)
