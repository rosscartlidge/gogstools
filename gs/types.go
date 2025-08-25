package gs

import (
	"context"
	"fmt"
	"reflect"
)

// FieldType represents the type of a command field
type FieldType string

const (
	FieldTypeString FieldType = "string"
	FieldTypeField  FieldType = "field"   // TSV field name
	FieldTypeFile   FieldType = "file"    // File path
	FieldTypeNumber FieldType = "number"  // Numeric value
	FieldTypeFlag   FieldType = "flag"    // Boolean flag
	FieldTypeMulti  FieldType = "multi"   // Multi-argument switch
)

// ArgumentType represents the type of an individual argument within a multi-argument switch
type ArgumentType string

const (
	ArgumentTypeString  ArgumentType = "string"
	ArgumentTypeField   ArgumentType = "field"   // TSV field name
	ArgumentTypeContent ArgumentType = "content" // Field content/values
	ArgumentTypeFile    ArgumentType = "file"    // File path
	ArgumentTypeNumber  ArgumentType = "number"  // Numeric value
)

// ArgumentSpec defines a single argument within a multi-argument switch
type ArgumentSpec struct {
	Name string       // Argument name (e.g., "field", "content")
	Type ArgumentType // Type of argument for completion
}

// FieldScope defines whether a field applies globally or per-clause
type FieldScope string

const (
	ScopeGlobal FieldScope = "global" // Applies to entire command
	ScopeLocal  FieldScope = "local"  // Applies within each clause
)

// FieldMode defines how multiple values are handled
type FieldMode string

const (
	ModeLast FieldMode = "last" // Only keep the last value
	ModeList FieldMode = "list" // Collect all values into a list
)

// FieldMeta contains metadata parsed from struct tags
type FieldMeta struct {
	Name         string        // Field name in struct
	Type         FieldType     // Type of field
	Scope        FieldScope    // Global or local scope
	Mode         FieldMode     // How to handle multiple values
	Args         []ArgumentSpec // For multi-argument switches
	DefaultValue interface{}   // Default value
	Help         string        // Help text
	Required     bool          // Whether field is required
	Complete     string        // Completion type hint
	Suffix       string        // File suffix filter for completion (e.g., ".tsv")
	Enum         []string      // Enumerated values for completion (e.g., ["bar", "line", "area"])
}

// ClauseSet represents a group of related arguments separated by + or -
type ClauseSet struct {
	Fields    map[string]interface{} // Parsed field values
	IsNegated bool                   // Whether this clause is negated (-)
}

// Commander interface for command execution
type Commander interface {
	Execute(ctx context.Context, clauses []ClauseSet) error
	Validate() error
}

// Completer interface for argument completion
type Completer interface {
	Complete(ctx context.Context, args []string, pos int) ([]string, error)
	CompleteField(filename, partial string) ([]string, error)
}

// DocumentGenerator interface for auto-documentation
type DocumentGenerator interface {
	GenerateHelp() string
	GenerateManPage() string
	GenerateUsage() string
}

// ParseError represents an error in command parsing
type ParseError struct {
	Field   string
	Value   string
	Message string
}

func (e ParseError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("field %s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %s: %s", e.Field, e.Message)
}

// reflectFields extracts field metadata from struct tags
func reflectFields(v interface{}) ([]FieldMeta, error) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %T", v)
	}
	
	typ := val.Type()
	fields := make([]FieldMeta, 0, typ.NumField())
	
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("gs")
		
		if tag == "" {
			continue // Skip fields without gs tags
		}
		
		meta, err := parseFieldTag(field.Name, tag)
		if err != nil {
			return nil, fmt.Errorf("parsing field %s: %w", field.Name, err)
		}
		
		fields = append(fields, meta)
	}
	
	return fields, nil
}