package lexer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

type Token int

const (
	EOF = iota
	ILLEGAL
	NUMBER
	IDENT

	USE      // use ...
	VAR      // var ...
	CTR      // container ...
	ENUM     // enum ...
	DEFINE   // define ...
	RESERVED // reserved ...

	STR_VALUE // "..."

	// types
	INT64
	INT32
	INT16
	INT

	UINT64
	UINT32
	UINT16
	UINT

	BYTES
	STRING

	FLOAT64
	FLOAT32

	BOOL
	BYTE
	// types

	UNSAFE // unsafe
	RCOPY  // return copy
	// type attributes

	OPEN_BRACKET  // [
	CLOSE_BRACKET // ]
	OPEN_BRACE    // {
	CLOSE_BRACE   // }

	OPEN_ARROW  // <
	CLOSE_ARROW // >

	COMMA     // ,
	EQUALS    // =
	SEMICOLON // ;

	// CUSTOM is appended last so adding it does not shift the integer values of
	// the type tokens above, which are serialized into existing BCD meta blocks.
	CUSTOM // custom ...
)

var tokens = []string{
	EOF:       "EOF",
	ILLEGAL:   "Illegal",
	NUMBER:    "Number",
	DEFINE:    "Define",
	RESERVED:  "Reserved",
	VAR:       "Var",
	USE:       "Use",
	IDENT:     "Identifier",
	STR_VALUE: "String Value",
	CTR:       "Container",
	ENUM:      "Enum",
	CUSTOM:    "Custom",

	INT64: "Int64",
	INT32: "Int32",
	INT16: "Int16",
	INT:   "Int",

	UINT64: "Uint64",
	UINT32: "Uint32",
	UINT16: "Uint16",
	UINT:   "Uint",

	FLOAT32: "Float32",
	FLOAT64: "Float64",

	BYTE: "Byte",
	BOOL: "Bool",

	BYTES:  "Bytes",
	STRING: "String",

	RCOPY:  "ReturnCopy",
	UNSAFE: "Unsafe",

	OPEN_BRACKET:  "[",
	CLOSE_BRACKET: "]",
	OPEN_BRACE:    "{",
	CLOSE_BRACE:   "}",

	OPEN_ARROW:  "<",
	CLOSE_ARROW: ">",

	COMMA:     ",",
	EQUALS:    "=",
	SEMICOLON: ";",
}

var keywords = map[string]Token{
	"reserved": RESERVED,
	"define":   DEFINE,
	"var":      VAR,
	"enum":     ENUM,
	"use":      USE,
	"ctr":      CTR,
	"custom":   CUSTOM,

	"int64": INT64,
	"int32": INT32,
	"int16": INT16,
	"int":   INT,

	"uint64": UINT64,
	"uint32": UINT32,
	"uint16": UINT16,
	"uint":   UINT,

	"float32": FLOAT32,
	"float64": FLOAT64,

	"bool": BOOL,
	"byte": BYTE,

	"bytes":  BYTES,
	"string": STRING,

	"unsafe": UNSAFE,
	"rcopy":  RCOPY,
}

func (t Token) String() string {
	return tokens[t]
}

func (t Token) Golang() string {
	switch t {
	case INT64:
		return "int64"
	case INT32:
		return "int32"
	case INT16:
		return "int16"
	case INT:
		return "int"
	case UINT64:
		return "uint64"
	case UINT32:
		return "uint32"
	case UINT16:
		return "uint16"
	case UINT:
		return "uint"
	case FLOAT32:
		return "float32"
	case FLOAT64:
		return "float64"
	case BYTE:
		return "byte"
	case BOOL:
		return "bool"
	case BYTES:
		return "[]byte"
	case STRING:
		return "string"
	}
	return "invalid type"
}

type Position struct {
	Line   int
	Column int
}

type Lexer struct {
	pos     Position
	reader  *bufio.Reader
	Content string

	// pendingComment holds the leading comment lines skipped since the last
	// emitted token; tokenComment is the leading comment block attached to the
	// most recently emitted token. trailingComment holds a comment found on the
	// same line as (i.e. after) the previous token — it belongs to the element
	// that ended on that line, not the next one.
	pendingComment  []string
	tokenComment    string
	trailingComment string
}

// Comment returns the leading comment attached to the token most recently
// returned by Lex (empty if it had none). Multi-line comment blocks are joined
// with "\n".
func (l *Lexer) Comment() string {
	return l.tokenComment
}

// TrailingComment returns a comment that appeared on the same line as the
// token preceding the one most recently returned by Lex (empty if none). It is
// discovered while scanning toward the next token, so it is read after the
// statement it trails has been parsed.
func (l *Lexer) TrailingComment() string {
	return l.trailingComment
}

func NewLexer(reader io.Reader, content string) *Lexer {
	return &Lexer{
		Content: content,
		pos:     Position{Line: 1, Column: 0},
		reader:  bufio.NewReader(reader),
	}
}

func (l *Lexer) error(message string) {
	errorMessage := "\n\033[1;31m[bencgen] Error:\033[0m\n"
	errorMessage += fmt.Sprintf("    \033[1;37m%d:%d\033[0m %s\n", l.pos.Line, l.pos.Column, highlightError(l.Content, l.pos.Line, l.pos.Column))
	errorMessage += fmt.Sprintf("    \033[1;37mMessage:\033[0m %s\n", message)
	fmt.Println(errorMessage)
	os.Exit(-1)
}

func highlightError(text string, lineNumber, columnNumber int) string {
	lines := strings.Split(text, "\n")
	if lineNumber <= 0 || lineNumber > len(lines) {
		return "Invalid line number <- report"
	}

	line := lines[lineNumber-1]
	if columnNumber <= 0 || columnNumber > len(line) {
		return "Invalid column number <- report"
	}

	highlightedLine := fmt.Sprintf("%s\033[1;31m%c\033[0m%s", line[:columnNumber], line[columnNumber], line[columnNumber+1:])
	arrow := strings.Repeat(" ", columnNumber-1+6+len(fmt.Sprintf("%d:%d", lineNumber, columnNumber))) + "\033[1;31m^\033[0m"
	return highlightedLine + "\n" + arrow
}

// Lex returns the next token, after recording any leading comment block in
// l.tokenComment (see Comment) and any same-line trailing comment from the
// previous token's line in l.trailingComment (see TrailingComment). It wraps
// lexRaw, which accumulates skipped comment lines.
func (l *Lexer) Lex() (Position, Token, string) {
	l.trailingComment = ""
	pos, tok, lit := l.lexRaw()
	l.tokenComment = strings.Join(l.pendingComment, "\n")
	l.pendingComment = l.pendingComment[:0]
	return pos, tok, lit
}

func (l *Lexer) lexRaw() (Position, Token, string) {
	comment := false
	// A comment is "trailing" (belongs to the previous token's line) when it
	// starts before any newline in this Lex call; once a newline is seen,
	// further comments are leading (belong to the upcoming token).
	trailing := false
	seenNewline := false
	var line strings.Builder

	flush := func() {
		if trailing {
			l.trailingComment = strings.TrimSpace(line.String())
		} else {
			l.pendingComment = append(l.pendingComment, strings.TrimSpace(line.String()))
		}
		line.Reset()
	}

	for {
		r, _, err := l.reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				if comment {
					flush()
				}
				return l.pos, EOF, ""
			}

			panic(err)
		}

		l.pos.Column++

		if r == '\n' {
			if comment {
				flush()
				comment = false
			}
			seenNewline = true
		}

		if comment {
			line.WriteRune(r)
			continue
		}

		switch r {
		case '\n':
			l.resetPosition()
		case '#':
			comment = true
			trailing = !seenNewline
			continue
		case '/':
			r2, _, err2 := l.reader.ReadRune()
			if err2 == nil && r2 == '/' {
				l.pos.Column++
				comment = true
				trailing = !seenNewline
				continue
			}
			if err2 == nil {
				l.backup()
			}
			return l.pos, ILLEGAL, string(r)
		case '[':
			return l.pos, OPEN_BRACKET, "["
		case ']':
			return l.pos, CLOSE_BRACKET, "]"
		case '{':
			return l.pos, OPEN_BRACE, "{"
		case '}':
			return l.pos, CLOSE_BRACE, "}"
		case '<':
			return l.pos, OPEN_ARROW, "<"
		case '>':
			return l.pos, CLOSE_ARROW, ">"
		case ',':
			return l.pos, COMMA, ","
		case '=':
			return l.pos, EQUALS, "="
		case ';':
			return l.pos, SEMICOLON, ";"
		case '"':
			var sb strings.Builder
			for {
				r, _, err := l.reader.ReadRune()
				if r == '\n' {
					l.error("String isn't valid (no end, expected: \"...\").")
				}
				if err != nil || r == '"' {
					break
				}
				l.pos.Column++
				sb.WriteRune(r)
			}
			return l.pos, STR_VALUE, sb.String()
		default:
			if unicode.IsSpace(r) {
				continue
			}

			if unicode.IsDigit(r) {
				startPos := l.pos
				l.backup()
				lit := l.lexNumber()
				return startPos, NUMBER, lit
			}

			if unicode.IsLetter(r) || r == '_' {
				startPos := l.pos
				l.backup()
				lit := l.lexIdent()
				if token, ok := keywords[lit]; ok {
					return startPos, token, lit
				}
				return startPos, IDENT, lit
			}

			return l.pos, ILLEGAL, string(r)
		}
	}
}

func (l *Lexer) resetPosition() {
	l.pos.Line++
	l.pos.Column = 0
}

func (l *Lexer) backup() {
	if err := l.reader.UnreadRune(); err != nil {
		panic(err)
	}
	l.pos.Column--
}

func (l *Lexer) lexNumber() string {
	var sb strings.Builder
	for {
		r, _, err := l.reader.ReadRune()
		if err != nil || !unicode.IsDigit(r) {
			l.backup()
			break
		}
		l.pos.Column++
		sb.WriteRune(r)
	}
	return sb.String()
}

func (l *Lexer) lexIdent() string {
	var sb strings.Builder
	for {
		r, _, err := l.reader.ReadRune()
		if err != nil || !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_') {
			l.backup()
			break
		}
		l.pos.Column++
		sb.WriteRune(r)
	}
	return sb.String()
}
