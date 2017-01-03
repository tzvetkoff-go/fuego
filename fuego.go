package fuego

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

// VERSION is the package version
const VERSION = "0.1.0"

// State constants
const (
	stateLiteral = iota
	stateOpenTag
	stateCloseTag
	stateCode
	stateDocString
	stateRegularString
)

// Code type constants
const (
	codeTypeCode = iota
	codeTypeHeader
	codeTypeSignature
	codeTypePrintEscaped
	codeTypePrintRaw
)

// ParseBytes parses a template from byte array
func ParseBytes(b []byte) ([]byte, error) {
	return ParseString(string(b))
}

// ParseString parses a template from string
func ParseString(s string) ([]byte, error) {
	return ParseRunes([]rune(s))
}

// ParseRunes parses a template from rune array
func ParseRunes(runes []rune) ([]byte, error) {
	// Default header block
	header := []string{}

	// Function signature
	signature := ""

	// Initial state and variables
	state := stateLiteral
	code := codeTypeCode
	htmlNeeded := false
	index := 0
	block := &bytes.Buffer{}
	output := &bytes.Buffer{}
	output.WriteString("fuegoOutput__ := &bytes.Buffer{}\n\n")

	// Walk template code
	count := len(runes)
	for index < count {
		current := runes[index]
		index++

		switch current {
		case '<':
			if state == stateLiteral {
				state = stateOpenTag
			} else {
				block.WriteRune(current)
			}
		case '%':
			if state == stateOpenTag {
				if block.Len() > 0 {
					output.WriteString("fuegoOutput__.WriteString(")
					output.WriteString(strconv.Quote(block.String()))
					output.WriteString(")\n")
					block.Reset()
				}

				state = stateCode
				code = codeTypeCode
			} else if state == stateCode {
				if block.Len() == 0 {
					code = codeTypeHeader
				} else {
					state = stateCloseTag
				}
			} else {
				block.WriteRune(current)
			}
		case '!':
			if state == stateCode && block.Len() == 0 {
				code = codeTypeSignature
			} else {
				block.WriteRune(current)
			}
		case '=':
			if state == stateCode && block.Len() == 0 {
				if code == codeTypePrintEscaped {
					code = codeTypePrintRaw
				} else {
					code = codeTypePrintEscaped
				}
			} else {
				block.WriteRune(current)
			}
		case '>':
			if state == stateCloseTag {
				if block.Len() > 0 {
					switch code {
					case codeTypeHeader:
						header = append(header, strings.TrimSpace(block.String()))
					case codeTypeSignature:
						signature = strings.TrimSpace(block.String())
					case codeTypePrintEscaped:
						output.WriteString("fuegoOutput__.WriteString(html.EscapeString(")
						output.ReadFrom(block)
						output.WriteString("))")
						htmlNeeded = true
					case codeTypePrintRaw:
						output.WriteString("fuegoOutput__.WriteString(")
						output.ReadFrom(block)
						output.WriteRune(')')
					default:
						output.ReadFrom(block)
					}

					output.WriteRune('\n')
					block.Reset()
				}

				state = stateLiteral
			} else {
				block.WriteRune(current)
			}
		case '"':
			if state == stateCode {
				state = stateRegularString
			} else if state == stateRegularString {
				state = stateCode
			}

			block.WriteRune(current)
		case '`':
			if state == stateCode {
				state = stateDocString
			} else if state == stateDocString {
				state = stateCode
			}

			block.WriteRune(current)
		case '\r', '\n':
			if state == stateLiteral && block.Len() > 0 {
				block.WriteRune(current)
			}
		default:
			if state == stateOpenTag {
				state = stateLiteral
				block.WriteRune('<')
			}

			block.WriteRune(current)
		}
	}

	// Simple check if we're inside a code block at EOF
	if state == stateCode || state == stateRegularString || state == stateDocString {
		return nil, errors.New("unterminated code block")
	}

	// Append last literal block
	if block.Len() > 0 {
		output.WriteString("fuegoOutput__.WriteString(")
		output.WriteString(strconv.Quote(block.String()))
		output.WriteString(")\n")
	}

	// Check header for html & bytes
	bytesFound, htmlFound := false, false
	for _, s := range header {
		bytesFound = bytesFound || strings.Contains(s, "\"bytes\"")
		htmlFound = htmlFound || strings.Contains(s, "\"html\"")
	}
	if !bytesFound {
		header = append(header, "import \"bytes\"")
	}
	if htmlNeeded && !htmlFound {
		header = append(header, "import \"html\"")
	}

	// Final code
	final := &bytes.Buffer{}

	// Add header
	final.WriteString(strings.Join(header, "\n"))
	final.WriteRune('\n')

	// Function signature
	final.WriteString("func ")
	final.WriteString(signature)
	final.WriteString(" string {\n")

	// Function code
	final.ReadFrom(output)

	// Result
	final.WriteString("\n\nreturn fuegoOutput__.String()\n}\n")

	return final.Bytes(), nil
}
