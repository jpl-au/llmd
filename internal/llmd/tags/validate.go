package tags

// Validate checks that a tag name is valid.
// Rules: lowercase alphanumeric and hyphens, max 64 characters.
func Validate(name string) error {
	if name == "" || len(name) > 64 {
		return ErrInvalid
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return ErrInvalid
		}
	}
	return nil
}
