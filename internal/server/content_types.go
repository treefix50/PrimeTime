package server

import "strings"

const (
	jsonContentType = "application/json; charset=utf-8"
	textContentType = "text/plain; charset=utf-8"
)

func normalizeTextContentType(contentType string) string {
	if contentType == "" {
		return textContentType
	}
	if strings.Contains(strings.ToLower(contentType), "charset=") {
		return contentType
	}
	return contentType + "; charset=utf-8"
}
