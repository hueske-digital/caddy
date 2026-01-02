package main

import "testing"

func TestExtractProjectName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"hyphen separator", "visual-studio-code-app-1", "visual-studio-code"},
		{"hyphen with simple name", "myproject-web-1", "myproject"},
		{"underscore separator", "my_project_web_1", "my_project"},
		{"single hyphen", "app-1", ""},
		{"no separator", "container", ""},
		{"complex name", "my-cool-project-backend-api-1", "my-cool-project-backend"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("extractProjectName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
