package validators

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = Timeout{}

// Timeout is a validator for timeout values
type Timeout struct {
	MaxAllowedDuration *time.Duration // Optional maximum duration, assumed to be 5h if not set
}

// Description returns a human-readable description of the validator
func (Timeout) Description(_ context.Context) string {
	return `Ensures a valid time value has been provided. "ns", "us", "ms", "s", "m", "h" are all legal.`
}

// MarkdownDescription returns a markdown-formatted description of the validator
func (Timeout) MarkdownDescription(_ context.Context) string {
	return `Ensures a valid time value has been provided. "ns", "us", "ms", "s", "m", "h" are all legal.`
}

// ValidateString validates the timeout value
func (t Timeout) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()
	if value == "" {
		return
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		response.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
			request.Path,
			"Invalid Timeout Value",
			`The timeout value must be a valid duration string. `+
				`For example: 5s, 1m30s, 2h45m.`,
		))
	}

	if d <= 0 {
		response.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
			request.Path,
			"Invalid Timeout Value",
			`The timeout value must be positive.`,
		))
	}

	maxDuration := 5 * time.Hour // Default maximum of 5 hours
	if t.MaxAllowedDuration != nil {
		maxDuration = *t.MaxAllowedDuration
	}

	if d > maxDuration {
		response.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
			request.Path,
			"Timeout Value Too Large",
			fmt.Sprintf(`The timeout value must be less than %s.`, maxDuration),
		))
	}
}
