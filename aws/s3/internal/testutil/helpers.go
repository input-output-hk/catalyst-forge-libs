// Package testutil provides test helper functions.
package testutil

import (
	"github.com/aws/aws-sdk-go-v2/aws"
)

// StringPtr returns a pointer to the given string.
// This is useful for AWS SDK inputs that require string pointers.
func StringPtr(s string) *string {
	return aws.String(s)
}

// Int64Ptr returns a pointer to the given int64.
// This is useful for AWS SDK inputs that require int64 pointers.
func Int64Ptr(i int64) *int64 {
	return aws.Int64(i)
}

// Int32Ptr returns a pointer to the given int32.
// This is useful for AWS SDK inputs that require int32 pointers.
func Int32Ptr(i int32) *int32 {
	return aws.Int32(i)
}

// BoolPtr returns a pointer to the given bool.
// This is useful for AWS SDK inputs that require bool pointers.
func BoolPtr(b bool) *bool {
	return aws.Bool(b)
}
