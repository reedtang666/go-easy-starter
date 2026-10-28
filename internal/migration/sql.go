package migration

import (
	"errors"
	"fmt"
	"strings"
)

type statement struct {
	text   string
	tokens []string
}

// prepareSQL permits a single legacy BEGIN/COMMIT wrapper, then leaves all
// transaction ownership to the runner. Literals, function bodies, identifiers
// and nested comments are scanned without treating their contents as SQL.
func prepareSQL(source string) (string, error) {
	statements, err := scanStatements(source)
	if err != nil {
		return "", err
	}
	if len(statements) == 0 {
		return "", errors.New("migration contains no executable SQL")
	}
	if isWrapper(statements[0].tokens, "BEGIN") {
		if len(statements) < 3 || !isWrapper(statements[len(statements)-1].tokens, "COMMIT") {
			return "", errors.New("migration BEGIN requires one outer COMMIT and executable SQL")
		}
		statements = statements[1 : len(statements)-1]
	}
	var out strings.Builder
	for _, s := range statements {
		if transactionCommand(s.tokens) {
			return "", errors.New("migration must not control transactions; only one outer BEGIN/COMMIT wrapper is allowed")
		}
		out.WriteString(s.text)
		out.WriteString(";\n")
	}
	return out.String(), nil
}

func isWrapper(tokens []string, command string) bool {
	return len(tokens) == 1 && tokens[0] == command || len(tokens) == 2 && tokens[0] == command && (tokens[1] == "WORK" || tokens[1] == "TRANSACTION")
}

func transactionCommand(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	switch tokens[0] {
	case "BEGIN", "COMMIT", "END", "ROLLBACK", "ABORT", "SAVEPOINT", "RELEASE":
		return true
	case "START", "PREPARE":
		return len(tokens) > 1 && tokens[1] == "TRANSACTION"
	case "SET":
		return len(tokens) > 1 && tokens[1] == "TRANSACTION" || len(tokens) > 4 && tokens[1] == "SESSION" && tokens[2] == "CHARACTERISTICS" && tokens[3] == "AS" && tokens[4] == "TRANSACTION"
	}
	return false
}

func scanStatements(source string) ([]statement, error) {
	var result []statement
	var tokens []string
	start := 0
	for i := 0; i < len(source); {
		c := source[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f':
			i++
		case c == '-' && i+1 < len(source) && source[i+1] == '-':
			i += 2
			for i < len(source) && source[i] != '\n' && source[i] != '\r' {
				i++
			}
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			i += 2
			depth := 1
			for i < len(source) && depth > 0 {
				if i+1 < len(source) && source[i:i+2] == "/*" {
					depth++
					i += 2
				} else if i+1 < len(source) && source[i:i+2] == "*/" {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return nil, errors.New("unterminated SQL block comment")
			}
		case c == '\'' || c == '"':
			quote := c
			escape := quote == '\'' && i > 0 && (source[i-1] == 'e' || source[i-1] == 'E') && (i < 2 || !identifierByte(source[i-2]))
			i++
			closed := false
			for i < len(source) {
				if escape && source[i] == '\\' {
					i += 2
					continue
				}
				if source[i] == quote {
					if i+1 < len(source) && source[i+1] == quote {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, errors.New("unterminated SQL string or identifier")
			}
			tokens = append(tokens, "<quoted>")
		case c == '$':
			end := i + 1
			for end < len(source) && (asciiLetter(source[end]) || source[end] >= 0x80 || source[end] == '_' || end > i+1 && source[end] >= '0' && source[end] <= '9') {
				end++
			}
			if end < len(source) && source[end] == '$' {
				tag := source[i : end+1]
				tail := strings.Index(source[end+1:], tag)
				if tail < 0 {
					return nil, errors.New("unterminated SQL dollar quote")
				}
				i = end + 1 + tail + len(tag)
				tokens = append(tokens, "<quoted>")
			} else {
				tokens = append(tokens, "$")
				i++
			}
		case c == ';':
			if len(tokens) != 0 {
				result = append(result, statement{text: source[start:i], tokens: tokens})
			}
			tokens = nil
			i++
			start = i
		case asciiLetter(c) || c == '_':
			end := i + 1
			for end < len(source) && identifierByte(source[end]) {
				end++
			}
			tokens = append(tokens, strings.ToUpper(source[i:end]))
			i = end
		default:
			tokens = append(tokens, string(c))
			i++
		}
	}
	if len(tokens) != 0 {
		result = append(result, statement{text: source[start:], tokens: tokens})
	}
	return result, nil
}

func asciiLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func identifierByte(c byte) bool {
	return asciiLetter(c) || c >= '0' && c <= '9' || c == '_' || c == '$' || c >= 0x80
}

func sqlError(filename string, err error) error {
	return fmt.Errorf("migration %s: %w", filename, err)
}
