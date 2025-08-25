package gs

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestConfig is a simple test configuration
type TestConfig struct {
	Name    string   `gs:"string,global,last,help=Name of the test,default=test"`
	Fields  []string `gs:"field,local,list,help=Field names"`
	Count   float64  `gs:"number,global,last,help=Number of items,default=10"`
	Verbose bool     `gs:"flag,global,last,help=Verbose output"`
}

func (tc *TestConfig) Execute(ctx context.Context, clauses []ClauseSet) error {
	return nil
}

func (tc *TestConfig) Validate() error {
	return nil
}

// TestCompletionConfig is a test config with various completion features
type TestCompletionConfig struct {
	Name      string                      `gs:"string,global,last,help=Name field"`
	Type      string                      `gs:"string,global,last,help=Type field,enum=bar:line:area,default=bar"`
	Field     string                      `gs:"field,global,last,help=Field name"`
	File      string                      `gs:"file,global,last,help=File path,suffix=.tsv"`
	DataFile  string                      `gs:"file,global,last,help=Data file,suffix=.[tc]sv"`
	Config    string                      `gs:"file,global,last,help=Config file,suffix=.{json,yaml}"`
	Match     []map[string]interface{}    `gs:"multi,local,list,args=field:content,help=Match conditions"`
}

func (tc *TestCompletionConfig) Execute(ctx context.Context, clauses []ClauseSet) error {
	return nil
}

func (tc *TestCompletionConfig) Validate() error {
	validTypes := map[string]bool{"bar": true, "line": true, "area": true}
	if !validTypes[tc.Type] {
		return fmt.Errorf("invalid type: %s", tc.Type)
	}
	return nil
}

func TestCommandCreation(t *testing.T) {
	config := &TestConfig{}
	cmd, err := NewCommand(config)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
	
	if len(cmd.fields) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(cmd.fields))
	}
}

func TestEnumCompletion(t *testing.T) {
	config := &TestCompletionConfig{}
	cmd, err := NewCommand(config)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		pos      int
		expected []string
	}{
		{
			name:     "enum completion - empty partial",
			args:     []string{"-type", ""},
			pos:      1,
			expected: []string{"bar", "line", "area"},
		},
		{
			name:     "enum completion - partial match 'l'",
			args:     []string{"-type", "l"},
			pos:      1,
			expected: []string{"line"},
		},
		{
			name:     "enum completion - partial match 'b'",
			args:     []string{"-type", "b"},
			pos:      1, 
			expected: []string{"bar"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			completions, err := cmd.complete(test.args, test.pos)
			if err != nil {
				t.Fatalf("Completion failed: %v", err)
			}

			if len(completions) != len(test.expected) {
				t.Errorf("Expected %d completions, got %d: %v", 
					len(test.expected), len(completions), completions)
				return
			}

			for i, expected := range test.expected {
				if i >= len(completions) || completions[i] != expected {
					t.Errorf("Expected completion[%d]='%s', got '%s'", 
						i, expected, completions[i])
				}
			}
		})
	}
}

func TestFlagCompletion(t *testing.T) {
	config := &TestCompletionConfig{}
	cmd, err := NewCommand(config)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		pos      int
		contains []string // Flags that should be present
	}{
		{
			name:     "flag completion - dash prefix",
			args:     []string{"-"},
			pos:      0,
			contains: []string{"-name", "-type", "-field", "-file", "-data-file", "-config", "-match"},
		},
		{
			name:     "flag completion - partial '-t'",
			args:     []string{"-t"},
			pos:      0,
			contains: []string{"-type"},
		},
		{
			name:     "negated flag completion",
			args:     []string{"+t"},
			pos:      0,
			contains: []string{"+type"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			completions, err := cmd.complete(test.args, test.pos)
			if err != nil {
				t.Fatalf("Completion failed: %v", err)
			}

			for _, expected := range test.contains {
				found := false
				for _, completion := range completions {
					if completion == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find '%s' in completions: %v", expected, completions)
				}
			}
		})
	}
}

func TestPositionHandling(t *testing.T) {
	config := &TestCompletionConfig{}
	cmd, err := NewCommand(config)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Test the specific position issue that broke completion
	tests := []struct {
		name        string
		args        []string
		pos         int
		expectError bool
		expectType  string
	}{
		{
			name:       "position 0 - flag completion",
			args:       []string{"-type"},
			pos:        0,
			expectType: "flag",
		},
		{
			name:       "position 1 - enum value completion", 
			args:       []string{"-type", "b"},
			pos:        1,
			expectType: "enum",
		},
		{
			name:        "position beyond bounds - should not crash",
			args:        []string{"-type"},
			pos:         5,
			expectError: false, // Should handle gracefully
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			completions, err := cmd.complete(test.args, test.pos)
			
			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// At minimum, should not crash and return some completions
			if test.expectType == "flag" && len(completions) == 0 {
				t.Errorf("Expected flag completions but got none")
			}
			
			if test.expectType == "enum" && len(completions) == 0 {
				t.Errorf("Expected enum completions but got none")
			}
		})
	}
}

func TestEnumParsing(t *testing.T) {
	// Test enum tag parsing
	tests := []struct {
		tag      string
		expected []string
	}{
		{
			tag:      "string,global,last,enum=bar:line:area",
			expected: []string{"bar", "line", "area"},
		},
		{
			tag:      "string,global,last,enum=single",
			expected: []string{"single"},
		},
		{
			tag:      "string,global,last,enum=one:two",
			expected: []string{"one", "two"},
		},
	}

	for _, test := range tests {
		meta, err := parseFieldTag("test", test.tag)
		if err != nil {
			t.Errorf("Failed to parse tag '%s': %v", test.tag, err)
			continue
		}

		if len(meta.Enum) != len(test.expected) {
			t.Errorf("Tag '%s': expected %d enum values, got %d: %v",
				test.tag, len(test.expected), len(meta.Enum), meta.Enum)
			continue
		}

		for i, expected := range test.expected {
			if meta.Enum[i] != expected {
				t.Errorf("Tag '%s': expected enum[%d]='%s', got '%s'",
					test.tag, i, expected, meta.Enum[i])
			}
		}
	}
}

func TestValidationIntegration(t *testing.T) {
	// Test that enum validation works with the command execution
	config := &TestCompletionConfig{}
	cmd, err := NewCommand(config)
	if err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorText   string
	}{
		{
			name:        "valid enum value",
			args:        []string{"-type", "bar"},
			expectError: false,
		},
		{
			name:        "invalid enum value",
			args:        []string{"-type", "invalid"},
			expectError: true,
			errorText:   "invalid type: invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clauses, err := cmd.Parse(test.args)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Apply clauses to config (simulate command execution)
			for _, clause := range clauses {
				for fieldName, value := range clause.Fields {
					switch fieldName {
					case "Type":
						if strVal, ok := value.(string); ok {
							config.Type = strVal
						}
					}
				}
			}

			err = config.Validate()
			if test.expectError {
				if err == nil {
					t.Errorf("Expected validation error but got none")
				} else if test.errorText != "" && !strings.Contains(err.Error(), test.errorText) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorText, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}