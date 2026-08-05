// Package mailaddr converts RFC 5322 mailbox addresses into display-safe text.
package mailaddr

import (
	"mime"
	"net/mail"
	"strings"
)

var wordDecoder = &mime.WordDecoder{}

// Format returns a readable mailbox address without re-encoding a Unicode
// display name as an RFC 2047 encoded-word. mail.Address.String is intended
// for wire-format headers, so it is not suitable for values exposed by the API.
func Format(name, address string) string {
	name = decodeWords(strings.TrimSpace(name))
	address = strings.TrimSpace(address)
	if name == "" {
		return address
	}
	if address == "" {
		return name
	}
	return name + " <" + address + ">"
}

// Normalize converts a previously stored wire-format address into readable
// display text. This keeps existing databases compatible after the parser fix.
func Normalize(value string) string {
	value = strings.TrimSpace(value)
	address, err := mail.ParseAddress(value)
	if err != nil {
		return decodeWords(value)
	}
	return Format(address.Name, address.Address)
}

// NormalizeList normalizes all addresses and preserves an empty non-nil list.
func NormalizeList(values []string) []string {
	if values == nil {
		return []string{}
	}
	for index, value := range values {
		values[index] = Normalize(value)
	}
	return values
}

func decodeWords(value string) string {
	decoded, err := wordDecoder.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}
