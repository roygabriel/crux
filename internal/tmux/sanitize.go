package tmux

import (
	"errors"
	"fmt"
	"strings"
)

// MaxInputLength is the maximum allowed byte length for send-keys input.
const MaxInputLength = 4096

// ErrInputTooLong is returned when input exceeds MaxInputLength.
var ErrInputTooLong = errors.New("input exceeds maximum length")

// ErrUnsafeInput is returned when input contains dangerous shell patterns.
var ErrUnsafeInput = errors.New("input contains unsafe characters")

// unsafePatterns lists shell metacharacter sequences that are rejected.
var unsafePatterns = []string{
	";",
	"&&",
	"||",
	"$(",
	"`",
	"\n",
}

// SanitizeInput validates that text is safe to send via tmux send-keys.
// It rejects inputs that exceed MaxInputLength or contain dangerous shell
// metacharacters (;, &&, ||, $(), backticks, newlines). Returns the
// original text unmodified if it passes validation.
// ValidateLength checks that text does not exceed MaxInputLength.
// Unlike SanitizeInput, it does not reject shell metacharacters.
// Use this for literal text delivery where tmux send-keys -l
// prevents shell interpretation.
func ValidateLength(text string) error {
	if len(text) > MaxInputLength {
		return fmt.Errorf("%w: %d bytes (max %d)", ErrInputTooLong, len(text), MaxInputLength)
	}
	return nil
}

func SanitizeInput(text string) (string, error) {
	if len(text) > MaxInputLength {
		return "", fmt.Errorf("%w: %d bytes (max %d)", ErrInputTooLong, len(text), MaxInputLength)
	}

	for _, pattern := range unsafePatterns {
		if strings.Contains(text, pattern) {
			return "", fmt.Errorf("%w: contains %q", ErrUnsafeInput, pattern)
		}
	}

	return text, nil
}
