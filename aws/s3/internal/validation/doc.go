// Package validation provides centralized input validation logic.
// This includes bucket name validation, object key validation, and security checks.
//
// All user inputs are validated before being sent to AWS to prevent
// injection attacks and ensure compliance with S3 requirements.
package validation
