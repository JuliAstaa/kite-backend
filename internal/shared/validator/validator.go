package validator

import "regexp"

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func IsValidHexColor(color string) bool {
	return hexColorPattern.MatchString(color)
}

func IsEmptyString(value string) bool {
	return value == ""
}
