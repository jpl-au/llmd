// content.go implements document content validation.
//
// Separated because content validation is intentionally minimal - we only
// check size, not format. Documents can contain any UTF-8 text (markdown,
// code, prose, data).
//
// Design: Only size is validated to prevent SQLite bloat from accidentally
// storing huge files. Content format is not validated because llmd is
// format-agnostic by design - it stores whatever you give it.

package validate

// Content validates document content size.
//
// Validation rules:
//   - Max length enforced if maxLen > 0 (0 means no limit)
//
// Note: Only size is validated, not content format. Documents can contain any
// UTF-8 text. The maxLen default (100MB via service config) prevents accidental
// storage of huge files that would bloat the SQLite database.
func Content(content string, maxLen int64) error {
	if maxLen > 0 && int64(len(content)) > maxLen {
		return ErrContentTooLarge
	}
	return nil
}
