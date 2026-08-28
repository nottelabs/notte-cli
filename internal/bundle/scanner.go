package bundle

import "strings"

// Stmt is one logical Python statement: a physical line plus any lines joined
// to it by an open bracket or a trailing backslash.
//
// Text has comments stripped and continuation newlines collapsed to single
// spaces, which is what the import parser wants. It is deliberately not
// faithful to the source; emission works from StartLine/EndLine against the
// original file so that everything the bundler does not rewrite survives
// byte-for-byte.
type Stmt struct {
	Text      string
	StartLine int // 1-based, inclusive
	EndLine   int // 1-based, inclusive
	Indent    int // leading space/tab count of the first physical line
}

// TopLevel reports whether the statement starts at column zero. Only
// column-zero statements bind module-level names, so everything the bundler
// analyses is filtered through this.
func (s Stmt) TopLevel() bool { return s.Indent == 0 }

// Scan splits Python source into logical statements.
//
// It is a tokenizer, not a parser: it understands strings, comments, brackets
// and continuations well enough to know where statements begin and end, and
// nothing else. That is sufficient because the bundler rejects every construct
// whose handling would need a real parse — see Bundle.
//
// Iteration is over bytes rather than runes on purpose. Every byte it reacts to
// is ASCII, and a UTF-8 continuation byte is never ASCII, so multi-byte
// characters pass through untouched.
func Scan(src string) []Stmt {
	var out []Stmt
	var buf strings.Builder

	line := 1
	depth := 0
	inStmt := false
	stmtStart := 0
	stmtIndent := 0
	atLineStart := true
	indent := 0

	var quote byte // 0 when not inside a string literal
	var triple bool

	flush := func(end int) {
		if !inStmt {
			return
		}
		if text := strings.TrimSpace(buf.String()); text != "" {
			out = append(out, Stmt{
				Text:      text,
				StartLine: stmtStart,
				EndLine:   end,
				Indent:    stmtIndent,
			})
		}
		buf.Reset()
		inStmt = false
	}

	for i := 0; i < len(src); i++ {
		c := src[i]

		if quote != 0 {
			buf.WriteByte(c)
			switch {
			case c == '\n':
				line++
			case c == '\\' && i+1 < len(src):
				// Backslash consumes the next byte in raw literals too — r"\""
				// is a two-character string, not an unterminated one — so the
				// prefix never changes where a string ends and is not parsed.
				i++
				buf.WriteByte(src[i])
				if src[i] == '\n' {
					line++
				}
			case c == quote && triple:
				if i+2 < len(src) && src[i+1] == quote && src[i+2] == quote {
					buf.WriteByte(src[i+1])
					buf.WriteByte(src[i+2])
					i += 2
					quote = 0
				}
			case c == quote:
				quote = 0
			}
			continue
		}

		if c == '\n' {
			if depth > 0 && inStmt {
				buf.WriteByte(' ')
			} else {
				flush(line)
			}
			line++
			atLineStart = true
			indent = 0
			continue
		}

		if atLineStart {
			if c == ' ' || c == '\t' {
				indent++
				continue
			}
			atLineStart = false
			if !inStmt {
				inStmt = true
				stmtStart = line
				stmtIndent = indent
			}
		}

		if c == '#' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			i-- // hand the newline back to the loop
			continue
		}

		if c == '\\' && i+1 < len(src) && src[i+1] == '\n' {
			i++
			line++
			atLineStart = true
			indent = 0
			buf.WriteByte(' ')
			continue
		}

		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '"', '\'':
			quote = c
			triple = i+2 < len(src) && src[i+1] == c && src[i+2] == c
			buf.WriteByte(c)
			if triple {
				buf.WriteByte(src[i+1])
				buf.WriteByte(src[i+2])
				i += 2
			}
			continue
		}

		buf.WriteByte(c)
	}
	flush(line)
	return out
}
