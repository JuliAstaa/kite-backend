package queryparam

import "strconv"

func ToInt(v string) (int, bool) {
	if v == "" {
		return 0, false
	}

	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}

	return parsed, true
}
