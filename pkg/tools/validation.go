// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"regexp"
)

// dns1123SubdomainRegex validates Kubernetes node names per RFC 1123.
// A DNS subdomain must:
// - Start with an alphanumeric character
// - End with an alphanumeric character
// - Contain only lowercase alphanumeric characters or '-'
// - Be at most 253 characters
var dns1123SubdomainRegex = regexp.MustCompile(
	`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`,
)

// maxNodeNameLength is the maximum length for a Kubernetes node name.
const maxNodeNameLength = 253

// isValidNodeName validates that a node name conforms to Kubernetes naming
// requirements (RFC 1123 DNS subdomain).
func isValidNodeName(name string) bool {
	if name == "" || len(name) > maxNodeNameLength {
		return false
	}
	return dns1123SubdomainRegex.MatchString(name)
}
