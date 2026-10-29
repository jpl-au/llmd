package tags

import "github.com/jpl-au/llmd/errs"

// Validate checks that a tag name is valid.
// Rules: lowercase alphanumeric and hyphens, max 64 characters.
func Validate(name string) error {
	if name == "" || len(name) > 64 {
		return invalid(name)
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return invalid(name)
		}
	}
	return nil
}

// invalid is ErrInvalid naming the rejected tag and the rule.
func invalid(name string) error {
	return errs.New(ErrInvalid, "%q is not a valid tag name. Use lowercase letters, digits and hyphens, up to 64 characters.", name)
}
