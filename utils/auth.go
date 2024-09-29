package utils

import (
	"encoding/base64"
	"strings"
)

// decodeBasicAuth decodes the Basic Authorization header
func DecodeBasicAuth(auth string) (string, error) {
	// Remove "Basic " prefix
	encoded := strings.TrimPrefix(auth, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
