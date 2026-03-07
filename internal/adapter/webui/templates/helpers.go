package templates

import "strconv"

// itoa converts an int to a string for use in templ components.
func itoa(n int) string {
	return strconv.Itoa(n)
}
