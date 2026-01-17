// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

// Package prompts provides MCP prompt definitions for GPU diagnostic workflows.
package prompts

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// PromptDef defines a prompt with its metadata and handler.
type PromptDef struct {
	// Name is the unique identifier for the prompt.
	Name string
	// Description is a human-readable description.
	Description string
	// Arguments defines the parameters the prompt accepts.
	Arguments []ArgumentDef
	// Template is the Go template for generating messages.
	Template string
}

// ArgumentDef defines a prompt argument.
type ArgumentDef struct {
	Name        string
	Description string
	Required    bool
	Default     string
}

// Handler is an alias for the server.PromptHandlerFunc type.
type Handler = server.PromptHandlerFunc

// ToMCPPrompt converts a PromptDef to an mcp.Prompt.
func (p *PromptDef) ToMCPPrompt() mcp.Prompt {
	opts := []mcp.PromptOption{
		mcp.WithPromptDescription(p.Description),
	}

	for _, arg := range p.Arguments {
		argOpts := []mcp.ArgumentOption{
			mcp.ArgumentDescription(arg.Description),
		}
		if arg.Required {
			argOpts = append(argOpts, mcp.RequiredArgument())
		}
		opts = append(opts, mcp.WithArgument(arg.Name, argOpts...))
	}

	return mcp.NewPrompt(p.Name, opts...)
}

// RenderTemplate renders the prompt template with provided arguments.
// Placeholders use the format {{key}} and are replaced with corresponding
// argument values. For placeholders defined in Arguments but not provided,
// the Default value is used. Undefined placeholders (not in Arguments) are
// left unchanged in the output.
func (p *PromptDef) RenderTemplate(args map[string]string) string {
	result := p.Template
	for key, value := range args {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	// Replace any remaining placeholders with defaults or empty
	for _, arg := range p.Arguments {
		placeholder := "{{" + arg.Name + "}}"
		if strings.Contains(result, placeholder) {
			if arg.Default != "" {
				result = strings.ReplaceAll(result, placeholder, arg.Default)
			} else {
				result = strings.ReplaceAll(result, placeholder, "")
			}
		}
	}
	return strings.TrimSpace(result)
}

// BuildHandler creates a standard handler for a PromptDef.
func (p *PromptDef) BuildHandler() Handler {
	return func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// Extract arguments
		args := make(map[string]string)
		for key, value := range req.Params.Arguments {
			args[key] = value
		}

		// Validate required arguments
		for _, arg := range p.Arguments {
			if arg.Required {
				if _, ok := args[arg.Name]; !ok {
					return nil, fmt.Errorf("missing required argument: %s", arg.Name)
				}
			}
		}

		// Render template
		content := p.RenderTemplate(args)

		// Build messages
		messages := []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(content)),
		}

		return mcp.NewGetPromptResult(p.Description, messages), nil
	}
}
