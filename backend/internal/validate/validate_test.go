package validate_test

import (
	"strings"
	"testing"

	"github.com/digithun/employee-portal/backend/internal/validate"
)

func TestMobile(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"thai local", "0812345678", "0812345678", false},
		{"thai local 9 digits", "021234567", "021234567", false},
		{"e164", "+66812345678", "+66812345678", false},
		{"strips separators", "081-234-5678", "0812345678", false},
		{"strips spaces and parens", "(081) 234 5678", "0812345678", false},
		{"empty clears the field", "", "", false},
		{"letters rejected", "08123abcde", "", true},
		{"too short rejected", "0812", "", true},
		{"too long rejected", "+6681234567890123", "", true},
		{"leading plus zero rejected", "+0812345678", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, msg := validate.Mobile(tc.in)
			if tc.wantErr {
				if msg == "" {
					t.Fatalf("Mobile(%q) accepted a bad number", tc.in)
				}
				return
			}
			if msg != "" {
				t.Fatalf("Mobile(%q) rejected a good number: %s", tc.in, msg)
			}
			if got != tc.want {
				t.Fatalf("Mobile(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEmail(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"user@example.com", "user@example.com", false},
		{"  User@Example.COM ", "user@example.com", false},
		{"first.last@sub.example.co.th", "first.last@sub.example.co.th", false},
		{"", "", true},
		{"not-an-email", "", true},
		{"no@domain", "", true},
		{"two@@at.com", "", true},
		{"spaces in@example.com", "", true},
		{strings.Repeat("a", 250) + "@example.com", "", true},
	}
	for _, tc := range cases {
		got, msg := validate.Email(tc.in)
		if tc.wantErr {
			if msg == "" {
				t.Errorf("Email(%q) should have failed", tc.in)
			}
			continue
		}
		if msg != "" {
			t.Errorf("Email(%q) failed: %s", tc.in, msg)
		}
		if got != tc.want {
			t.Errorf("Email(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAddress(t *testing.T) {
	if _, msg := validate.Address("  123 Sukhumvit Rd  "); msg != "" {
		t.Errorf("valid address rejected: %s", msg)
	}
	if got, _ := validate.Address("  trimmed  "); got != "trimmed" {
		t.Errorf("address not trimmed: %q", got)
	}
	if _, msg := validate.Address(""); msg != "" {
		t.Errorf("empty address should clear the field, got %s", msg)
	}
	if _, msg := validate.Address(strings.Repeat("x", 501)); msg == "" {
		t.Error("501-character address should be rejected")
	}
	// Length is counted in runes, so 500 Thai characters must still pass.
	if _, msg := validate.Address(strings.Repeat("ก", 500)); msg != "" {
		t.Errorf("500 Thai characters should be accepted, got %s", msg)
	}
}

func TestSkillSelection(t *testing.T) {
	got, msg := validate.SkillSelection([]string{"a", "b", "a", "", "  c  "})
	if msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("SkillSelection deduped to %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SkillSelection = %v, want %v", got, want)
		}
	}

	tooMany := make([]string, validate.MaxSkills+1)
	for i := range tooMany {
		tooMany[i] = string(rune('a'+i%26)) + string(rune('0'+i/26))
	}
	if _, msg := validate.SkillSelection(tooMany); msg == "" {
		t.Error("more than 50 skills should be rejected")
	}
}

func TestErrorsKeepsFirstMessage(t *testing.T) {
	errs := validate.Errors{}
	if errs.Any() {
		t.Error("a fresh Errors map should be empty")
	}
	errs.Add("mobile", "first")
	errs.Add("mobile", "second")
	if errs["mobile"] != "first" {
		t.Errorf("Add overwrote the first message: %q", errs["mobile"])
	}
	if !errs.Any() {
		t.Error("Any() should be true after Add")
	}
}

func TestSkillName(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"trims", "  Rust  ", "Rust", false},
		{"keeps inner punctuation", "CI/CD", "CI/CD", false},
		{"blank rejected", "   ", "", true},
		{"too long rejected", strings.Repeat("a", 61), "", true},
		{"exactly 60 accepted", strings.Repeat("a", 60), strings.Repeat("a", 60), false},
		// A comma would be split back into two skills when a change is undone,
		// because history stores a selection as a comma-joined list.
		{"comma rejected", "React, Native", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, msg := validate.SkillName(tc.in)
			if tc.wantErr {
				if msg == "" {
					t.Fatalf("SkillName(%q) should have failed", tc.in)
				}
				return
			}
			if msg != "" {
				t.Fatalf("SkillName(%q) failed: %s", tc.in, msg)
			}
			if got != tc.want {
				t.Errorf("SkillName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCustomSkills(t *testing.T) {
	got, msg := validate.CustomSkills([]string{"  Elixir  ", "elixir", "Zig", "", "   "})
	if msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	want := []string{"Elixir", "Zig"}
	if len(got) != len(want) {
		t.Fatalf("CustomSkills = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CustomSkills = %v, want %v (first spelling wins)", got, want)
		}
	}
}

func TestCustomSkillsRejectsABadName(t *testing.T) {
	if _, msg := validate.CustomSkills([]string{"fine", "React, Native"}); msg == "" {
		t.Error("a comma in any entry should be rejected")
	}
	if _, msg := validate.CustomSkills([]string{strings.Repeat("a", 61)}); msg == "" {
		t.Error("an over-long entry should be rejected")
	}
}

func TestCustomSkillsHandlesAnEmptyList(t *testing.T) {
	got, msg := validate.CustomSkills(nil)
	if msg != "" || len(got) != 0 {
		t.Errorf("CustomSkills(nil) = %v, %q", got, msg)
	}
}
