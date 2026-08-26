package persistence

import (
	"strings"
	"testing"
)

func TestSplitSQL_Basic(t *testing.T) {
	got := splitSQL("SELECT 1; SELECT 2;")
	if len(got) != 2 {
		t.Fatalf("splitSQL = %d statements, want 2: %q", len(got), got)
	}
	if strings.TrimSpace(got[0]) != "SELECT 1" || strings.TrimSpace(got[1]) != "SELECT 2" {
		t.Errorf("splitSQL = %q", got)
	}
}

func TestSplitSQL_SemicolonInString(t *testing.T) {
	got := splitSQL("INSERT INTO t VALUES ('a;b');")
	if len(got) != 1 {
		t.Fatalf("splitSQL = %d statements, want 1: %q", len(got), got)
	}
	if !strings.Contains(got[0], "'a;b'") {
		t.Errorf("string content lost: %q", got[0])
	}
}

func TestSplitSQL_SemicolonInLineComment(t *testing.T) {
	got := splitSQL("SELECT 1; -- note; with semicolon\nSELECT 2;")
	if len(got) != 2 {
		t.Fatalf("splitSQL = %d statements, want 2: %q", len(got), got)
	}
}

func TestSplitSQL_SemicolonInBlockComment(t *testing.T) {
	got := splitSQL("SELECT 1; /* block; comment */ SELECT 2;")
	if len(got) != 2 {
		t.Fatalf("splitSQL = %d statements, want 2: %q", len(got), got)
	}
	if strings.Contains(got[1], "block") {
		t.Errorf("block comment not stripped: %q", got[1])
	}
}

func TestSplitSQL_EscapedQuote(t *testing.T) {
	got := splitSQL("INSERT INTO t VALUES ('it''s');")
	if len(got) != 1 {
		t.Fatalf("splitSQL = %d statements, want 1: %q", len(got), got)
	}
}

func TestSplitSQL_TrailingNoSemicolon(t *testing.T) {
	got := splitSQL("SELECT 1; SELECT 2")
	if len(got) != 2 {
		t.Fatalf("splitSQL = %d statements, want 2: %q", len(got), got)
	}
}

func TestSplitSQL_OnlySeparators(t *testing.T) {
	got := splitSQL(";;  ;;")
	if len(got) != 0 {
		t.Fatalf("splitSQL = %d statements, want 0: %q", len(got), got)
	}
}
