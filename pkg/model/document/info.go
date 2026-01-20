package document

// Info contains document metadata without content, used for listings.
type Info struct {
	Key       string
	Path      string
	Version   int
	Author    string
	Message   string
	Source    string
	MIME      string
	Meta      *Meta
	CreatedAt int64
	DeletedAt *int64
}
