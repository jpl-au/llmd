package document

// Meta holds computed document metadata.
//
// Meta is populated when documents are written and provides quick access
// to content statistics without parsing the content again. Stored as JSON
// in the database.
type Meta struct {
	// Size is the content length in bytes.
	Size int `json:"size"`

	// Lines is the number of lines in the content.
	// Useful for display and pagination.
	Lines int `json:"lines"`
}
