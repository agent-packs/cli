// Package merge injects and retracts pack-owned fragments into shared files the
// pack does not own (an agent's memory markdown, or its JSON settings). Every
// operation is idempotent and records exactly what it added so it can later be
// cleanly retracted, leaving surrounding user content untouched.
package merge

import (
	"crypto/sha256"
	"fmt"
)

// Result reports the outcome of an Apply operation.
type Result struct {
	// Changed is true when the file content was modified.
	Changed bool
	// ContentHash is the sha256 of the injected fragment, recorded for drift
	// detection (so a later edit inside the managed region is detectable).
	ContentHash string
	// OwnedKeys lists the dotted key paths a structured (JSON) merge added; it
	// is empty for markdown merges, which are identified by their block marker.
	OwnedKeys []string
}

// HashString returns the sha256 of s as a hex string with a "sha256:" prefix.
func HashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("sha256:%x", sum)
}
