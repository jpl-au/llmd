package search

import (
	"regexp/syntax"
	"strings"
)

// ExtractSearchTerms extracts FTS5-searchable literal terms from a regex pattern.
// Returns nil if no useful terms can be extracted (e.g., pattern is too complex
// or contains metacharacters).
//
// The extracted terms can be used to pre-filter documents using FTS5 before
// applying the full regex match, significantly improving search performance.
func ExtractSearchTerms(pattern string) []string {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}

	var terms []string
	extractLiterals(re, &terms)

	// Filter out short terms (less useful for FTS)
	var result []string
	for _, t := range terms {
		if len(t) >= 3 {
			result = append(result, t)
		}
	}

	return result
}

// extractLiterals recursively extracts literal strings from a parsed regex.
func extractLiterals(re *syntax.Regexp, terms *[]string) {
	switch re.Op {
	case syntax.OpLiteral:
		// Direct literal string
		s := string(re.Rune)
		if s != "" {
			*terms = append(*terms, strings.ToLower(s))
		}

	case syntax.OpConcat:
		// Concatenation: try to build longer literals from adjacent literal nodes
		var builder strings.Builder
		for _, sub := range re.Sub {
			if sub.Op == syntax.OpLiteral {
				builder.WriteString(string(sub.Rune))
			} else {
				// Flush accumulated literal
				if builder.Len() > 0 {
					*terms = append(*terms, strings.ToLower(builder.String()))
					builder.Reset()
				}
				// Recurse into non-literal
				extractLiterals(sub, terms)
			}
		}
		// Flush any remaining literal
		if builder.Len() > 0 {
			*terms = append(*terms, strings.ToLower(builder.String()))
		}

	case syntax.OpCapture:
		// Capture group: recurse into content
		for _, sub := range re.Sub {
			extractLiterals(sub, terms)
		}

	case syntax.OpAlternate:
		// Alternation (|): extract from all branches
		for _, sub := range re.Sub {
			extractLiterals(sub, terms)
		}

	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		// Quantifiers: recurse but don't include in terms since they're optional/repeated
		// We still extract literals for pre-filtering purposes
		for _, sub := range re.Sub {
			extractLiterals(sub, terms)
		}
	}
}

// BuildFTSQuery builds an FTS5 query from extracted terms.
// Returns empty string if no suitable query can be built.
func BuildFTSQuery(terms []string) string {
	if len(terms) == 0 {
		return ""
	}

	// Use OR to match any of the terms
	// FTS5 will return documents containing any of these terms
	var parts []string
	for _, t := range terms {
		// Quote the term to treat it as a literal phrase
		parts = append(parts, `"`+escapeFTS(t)+`"`)
	}

	return strings.Join(parts, " OR ")
}

// escapeFTS escapes special characters for FTS5 queries.
func escapeFTS(s string) string {
	// FTS5 special characters that need escaping in quoted strings
	s = strings.ReplaceAll(s, `"`, `""`)
	return s
}
