// audit.go defines the internal Audit type and scan helpers.

package audits

import (
	"database/sql"
)

// Audit is the internal representation of an audit record.
type Audit struct {
	ID         string
	Target     string
	TargetType string
	Version    int
	Author     string
	Status     string
	Content    string
	ParentID   string
	CreatedAt  int64
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanAudit reads an audit from any scanner implementation.
func scanAudit(s scanner) (*Audit, error) {
	var a Audit
	var version sql.NullInt64
	var parentID sql.NullString
	var deletedAt sql.NullInt64

	err := s.Scan(
		&a.ID, &a.Target, &a.TargetType, &version,
		&a.Author, &a.Status, &a.Content, &parentID,
		&a.CreatedAt, &deletedAt,
	)
	if err != nil {
		return nil, err
	}

	if version.Valid {
		a.Version = int(version.Int64)
	}
	if parentID.Valid {
		a.ParentID = parentID.String
	}
	return &a, nil
}

const columns = `id, target, target_type, version, author, status, content, parent_id, created_at, deleted_at`
