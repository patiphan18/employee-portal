// Package validate holds the field rules from FR-3.4 to FR-3.7. It is deliberately free
// of HTTP and database concerns so the rules can be unit tested on their own.
package validate

import (
	"regexp"
	"strings"
)

const (
	MaxAddressLen = 500
	MaxEmailLen   = 254
	MaxSkills     = 50
)

var (
	// E.164 (+66812345678) or Thai local (0812345678).
	phoneRe = regexp.MustCompile(`^(\+[1-9]\d{8,14}|0\d{8,9})$`)
	emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@.]+(\.[^\s@.]+)+$`)
)

// Errors maps a field name to its first failure. A nil or empty map means valid.
type Errors map[string]string

func (e Errors) Add(field, msg string) {
	if _, exists := e[field]; !exists {
		e[field] = msg
	}
}

func (e Errors) Any() bool { return len(e) > 0 }

// NormalizeMobile trims spaces, dashes and parentheses so that "081-234-5678" and
// "0812345678" are treated as the same number.
func NormalizeMobile(s string) string {
	r := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "")
	return r.Replace(strings.TrimSpace(s))
}

// Mobile validates a normalized phone number. An empty string is valid: it clears the
// field.
func Mobile(s string) (string, string) {
	s = NormalizeMobile(s)
	if s == "" {
		return "", ""
	}
	if !phoneRe.MatchString(s) {
		return s, "invalid phone number (use +66812345678 or 0812345678)"
	}
	return s, ""
}

// Email validates and lowercases an address. Empty is rejected: a user must keep a
// login address.
func Email(s string) (string, string) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s, "email is required"
	}
	if len(s) > MaxEmailLen {
		return s, "email is too long"
	}
	if !emailRe.MatchString(s) {
		return s, "invalid email address"
	}
	return s, ""
}

// Address validates free text. Empty is valid: it clears the field.
func Address(s string) (string, string) {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > MaxAddressLen {
		return s, "address must be at most 500 characters"
	}
	return s, ""
}

// SkillSelection checks the shape of a skill id list before the catalog is consulted.
// It returns the ids with duplicates and blanks removed.
func SkillSelection(ids []string) ([]string, string) {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) > MaxSkills {
		return out, "at most 50 skills can be selected"
	}
	return out, ""
}

const MaxSkillNameLen = 60

// SkillName validates a skill name, for the admin catalog (FR-4.2) and for an
// employee's own free-text skill (FR-3.10) alike.
//
// Commas are rejected on purpose: the audit log stores a skill selection as a
// comma-joined list of names, so a name containing a comma would be split back into
// the wrong skills when the change is undone.
func SkillName(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return s, "name is required"
	}
	if len([]rune(s)) > MaxSkillNameLen {
		return s, "name must be at most 60 characters"
	}
	if strings.ContainsRune(s, ',') {
		return s, "name must not contain a comma"
	}
	return s, ""
}

// CustomSkills cleans a list of free-text skill names typed by an employee. It trims,
// drops blanks, removes case-insensitive duplicates, and keeps the first spelling the
// employee used. The returned error message, if any, refers to the whole list.
func CustomSkills(names []string) ([]string, string) {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, raw := range names {
		name, msg := SkillName(raw)
		if name == "" && msg == "name is required" {
			continue // a blank entry is simply ignored
		}
		if msg != "" {
			return out, msg
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out, ""
}
