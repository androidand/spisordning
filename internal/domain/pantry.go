package domain

import (
	"slices"
	"strings"
)

// Confidence is how sure the system is that an InventoryLot's current
// recorded quantity/existence reflects physical reality. See
// openspec/changes/implement-pantry-inventory/design.md D3 — exactly one of
// these four tiers, never a free-form value (invariant 4).
type Confidence string

const (
	ConfidenceExact     Confidence = "EXACT"
	ConfidenceLikely    Confidence = "LIKELY"
	ConfidenceEstimated Confidence = "ESTIMATED"
	ConfidenceUnknown   Confidence = "UNKNOWN"
)

// EventKind is one of implement-pantry-inventory's six inventory_event kinds
// (design.md D7 — MARK_EMPTY collapses into CONSUME with source:
// "mark_empty" rather than being its own kind).
type EventKind string

const (
	EventPurchase EventKind = "PURCHASE"
	EventConsume  EventKind = "CONSUME"
	EventDiscard  EventKind = "DISCARD"
	EventAdjust   EventKind = "ADJUST"
	EventTransfer EventKind = "TRANSFER"
	EventOpen     EventKind = "OPEN"
)

// SourceMarkEmpty is the source value RecordMarkEmpty writes on the CONSUME
// event it emits (design.md D7) — always resolves to ConfidenceExact
// regardless of the estimated flag (design.md D3: "zero is a certain
// observation").
const SourceMarkEmpty = "mark_empty"

// ConfidenceForEvent implements design.md D3's deterministic
// (event kind, source) → confidence transition table. It is defined only for
// the event kinds that actually set a new confidence on write — PURCHASE,
// CONSUME, DISCARD, ADJUST. TRANSFER and OPEN are deliberately not handled
// here: TRANSFER preserves the source lot's confidence unchanged, and OPEN
// does not by itself change confidence (it only marks the lot eligible for
// future time-based decay) — both are the caller's responsibility to leave
// alone, not something this function can decide from (kind, source) alone.
//
// estimated distinguishes an explicit counted quantity (false) from a
// household member's estimate like "about half" (true); PURCHASE quantities
// are always treated as known (the design gives no "estimated purchase"
// case), so estimated is ignored for EventPurchase.
func ConfidenceForEvent(kind EventKind, source string, estimated bool) Confidence {
	if kind == EventConsume && source == SourceMarkEmpty {
		return ConfidenceExact
	}
	switch kind {
	case EventPurchase:
		return ConfidenceExact
	case EventConsume, EventDiscard, EventAdjust:
		if estimated {
			return ConfidenceEstimated
		}
		return ConfidenceExact
	default:
		// TRANSFER/OPEN/unrecognized: no confidence transition defined here: see
		// the doc comment above. Return the "no belief" tier rather than silently
		// asserting EXACT for a kind this function doesn't actually reason about.
		return ConfidenceUnknown
	}
}

// NormalizeGTIN validates a barcode's check digit and canonicalizes it to a
// 14-digit GTIN-14 form (left-zero-padded), per implement-pantry-inventory
// design.md D6/Step 7 task 7.1. Accepts GTIN-8/12/13/14 lengths; non-digit
// characters (spaces, dashes sometimes present in scanned/typed input) are
// stripped before validation.
func NormalizeGTIN(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch len(d) {
	case 8, 12, 13, 14:
	default:
		return "", errInvalidGTIN(raw, "must be 8, 12, 13, or 14 digits after stripping non-digits")
	}
	if !validGTINCheckDigit(d) {
		return "", errInvalidGTIN(raw, "invalid check digit")
	}
	return strings.Repeat("0", 14-len(d)) + d, nil
}

// validGTINCheckDigit implements the standard GS1 check-digit algorithm:
// starting from the rightmost digit (the check digit itself, excluded from
// the sum) and working left across the remaining digits, weights alternate
// 3, 1, 3, 1, ... The check digit is (10 - sum%10) % 10.
func validGTINCheckDigit(d string) bool {
	sum := 0
	weight := 3
	for i := len(d) - 2; i >= 0; i-- {
		digit := int(d[i] - '0')
		sum += digit * weight
		if weight == 3 {
			weight = 1
		} else {
			weight = 3
		}
	}
	check := (10 - sum%10) % 10
	return check == int(d[len(d)-1]-'0')
}

type gtinError struct {
	raw    string
	reason string
}

func (e gtinError) Error() string {
	return "domain: invalid GTIN " + e.raw + ": " + e.reason
}

func errInvalidGTIN(raw, reason string) error {
	return gtinError{raw: raw, reason: reason}
}

// WouldCreateLocationCycle reports whether setting childID's parent to
// candidateParentID would create a cycle in the inventory_location tree
// (implement-pantry-inventory design.md D9, invariant 8), mirroring
// internal/recipefamily.Graph.AddEdge's cycle-prevention shape for a simpler
// case (a location has at most one parent — a tree, not a DAG).
//
// The caller is responsible for supplying candidateParentAncestors: the full
// ancestor chain of candidateParentID (its parent, grandparent, ... up to the
// root), typically loaded with a single recursive query. A cycle exists iff
// childID is candidateParentID itself, or childID already appears among
// candidateParentID's ancestors (childID would become an ancestor of its own
// ancestor).
func WouldCreateLocationCycle(childID, candidateParentID InventoryLocationID, candidateParentAncestors []InventoryLocationID) bool {
	if childID == candidateParentID {
		return true
	}
	return slices.Contains(candidateParentAncestors, childID)
}
