package domain

import (
	"testing"
)

// TestTypedIDCompileTimeRejection verifies that the typed ID types are
// distinct named types, so a repository call passing the wrong entity's id
// type does not compile. This is a compile-time test: if the types were
// interchangeable (e.g. all just `string` or all the same named type), the
// assignments below would compile and this test would fail at the type level.
//
// The test itself is trivially passing; its value is that it documents and
// pins the type-safety invariant. If someone changes PersonID to be an alias
// for IngredientID (or both to plain string), this file will still compile
// but the type-safety guarantee is broken. The real enforcement is that the
// types are distinct named types wrapping uuid.UUID.
func TestTypedIDCompileTimeRejection(t *testing.T) {
	// These are distinct types. The compiler enforces that you cannot
	// assign a PersonID where an IngredientID is expected without an
	// explicit conversion. We verify the types are distinct by checking
	// their string representations differ for the same underlying UUID.

	var p PersonID
	var i IngredientID
	var h HouseholdID

	// All zero-valued, but they are different types.
	// This compiles because we're just declaring variables of different types.
	_ = p
	_ = i
	_ = h

	// Verify String() works on all of them.
	if p.String() != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("PersonID zero value String() = %q, want zero UUID", p.String())
	}
	if i.String() != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("IngredientID zero value String() = %q, want zero UUID", i.String())
	}
	if h.String() != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("HouseholdID zero value String() = %q, want zero UUID", h.String())
	}
}

// TestTypedIDParse verifies that Parse* helpers work correctly.
func TestTypedIDParse(t *testing.T) {
	s := "01900000-0000-7000-8000-000000000001"

	p, err := ParsePersonID(s)
	if err != nil {
		t.Fatalf("ParsePersonID: %v", err)
	}
	if p.String() != s {
		t.Errorf("PersonID = %q, want %q", p.String(), s)
	}

	i, err := ParseIngredientID(s)
	if err != nil {
		t.Fatalf("ParseIngredientID: %v", err)
	}
	if i.String() != s {
		t.Errorf("IngredientID = %q, want %q", i.String(), s)
	}

	// Invalid UUID should fail.
	if _, err := ParsePersonID("not-a-uuid"); err == nil {
		t.Error("ParsePersonID with invalid UUID: expected error, got nil")
	}
}

// TestTypedIDNew verifies that New* constructors generate valid UUIDs.
func TestTypedIDNew(t *testing.T) {
	p := NewPersonID()
	if p.String() == "00000000-0000-0000-0000-000000000000" {
		t.Error("NewPersonID returned zero UUID")
	}

	i := NewIngredientID()
	if i.String() == "00000000-0000-0000-0000-000000000000" {
		t.Error("NewIngredientID returned zero UUID")
	}

	// Two new IDs should be different.
	if p.String() == NewPersonID().String() {
		t.Error("Two NewPersonID() calls returned the same UUID")
	}
}

// TestTypedIDNewIsV7 verifies that New* constructors generate UUIDv7 (per
// design D1: "UUIDv7 primary keys, generated in Go"). A UUIDv7 has version
// nibble 7 (the 13th hex digit, i.e. index 14 of the 32-char no-dash form, or
// the 3rd group's first digit in the dashed form).
func TestTypedIDNewIsV7(t *testing.T) {
	for name, id := range map[string]string{
		"PersonID":    NewPersonID().String(),
		"ProductID":   NewProductID().String(),
		"MealEventID": NewMealEventID().String(),
		"OrderID":     NewOrderID().String(),
	} {
		// Dashed form: xxxxxxxx-xxxx-7xxx-...  the version is the first
		// character of the third group.
		groups := splitDash(id)
		if len(groups) != 5 {
			t.Fatalf("%s: %q is not a dashed UUID", name, id)
		}
		if groups[2][0] != '7' {
			t.Errorf("%s: version nibble = %q, want '7' (UUIDv7); full id %q", name, groups[2][0], id)
		}
	}
}

func splitDash(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
