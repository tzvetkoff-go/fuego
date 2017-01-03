package main

import (
	"bytes"
	"html"
)

func FooBar(s string) string {
	fuegoOutput__ := &bytes.Buffer{}

	fuegoOutput__.WriteString("<h1>Escaped, ")
	fuegoOutput__.WriteString(html.EscapeString(s))
	fuegoOutput__.WriteString("</h1>\n<h1>Raw, ")
	fuegoOutput__.WriteString(s)
	fuegoOutput__.WriteString("</h1>\n")

	return fuegoOutput__.String()
}
