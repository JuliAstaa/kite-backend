package validator

import (
	"regexp"
	"strings"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidHexColor(color string) bool {
	return hexColorPattern.MatchString(color)
}

// IsValidUUID dipakai supaya id ngawur ditolak di handler dengan 400,
// bukan sampai ke Postgres dan balik jadi 500.
func IsValidUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

func IsEmptyString(value string) bool {
	return value == ""
}

// IsBlank menganggap string yang isinya cuma spasi sebagai kosong.
func IsBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func IsOneOf(value string, allowed ...string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	return false
}
