package gs

import (
	"fmt"
	"strconv"
	"strings"
)

// parseFieldTag parses a struct tag into FieldMeta
// Format: "type,scope,mode,key=value,key=value,..."
// Example: "field,global,last,help=Use field for X axis,default=time"
func parseFieldTag(fieldName, tag string) (FieldMeta, error) {
	meta := FieldMeta{
		Name:  fieldName,
		Type:  FieldTypeString, // Default
		Scope: ScopeGlobal,     // Default
		Mode:  ModeLast,        // Default
	}
	
	parts := parseTagParts(tag)
	if len(parts) == 0 {
		return meta, fmt.Errorf("empty tag")
	}
	
	// Parse positional parts
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Check for key=value pairs
		if strings.Contains(part, "=") {
			if err := parseKeyValue(part, &meta); err != nil {
				return meta, err
			}
			continue
		}
		
		// Handle positional arguments
		switch i {
		case 0: // Type
			if err := parseFieldType(part, &meta); err != nil {
				return meta, err
			}
		case 1: // Scope
			if err := parseFieldScope(part, &meta); err != nil {
				return meta, err
			}
		case 2: // Mode
			if err := parseFieldMode(part, &meta); err != nil {
				return meta, err
			}
		default:
			// Additional positional args treated as key=value if they contain =
			if strings.Contains(part, "=") {
				if err := parseKeyValue(part, &meta); err != nil {
					return meta, err
				}
			}
		}
	}
	
	return meta, nil
}

func parseFieldType(s string, meta *FieldMeta) error {
	switch s {
	case "string":
		meta.Type = FieldTypeString
	case "field":
		meta.Type = FieldTypeField
	case "file":
		meta.Type = FieldTypeFile
	case "number":
		meta.Type = FieldTypeNumber
	case "flag":
		meta.Type = FieldTypeFlag
	case "multi":
		meta.Type = FieldTypeMulti
	default:
		return fmt.Errorf("unknown field type: %s", s)
	}
	return nil
}

func parseFieldScope(s string, meta *FieldMeta) error {
	switch s {
	case "global":
		meta.Scope = ScopeGlobal
	case "local":
		meta.Scope = ScopeLocal
	default:
		return fmt.Errorf("unknown field scope: %s", s)
	}
	return nil
}

func parseFieldMode(s string, meta *FieldMeta) error {
	switch s {
	case "last":
		meta.Mode = ModeLast
	case "list":
		meta.Mode = ModeList
	default:
		return fmt.Errorf("unknown field mode: %s", s)
	}
	return nil
}

func parseKeyValue(kv string, meta *FieldMeta) error {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid key=value pair: %s", kv)
	}
	
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	
	switch key {
	case "help":
		meta.Help = value
	case "default":
		meta.DefaultValue = parseDefaultValue(value, meta.Type)
	case "required":
		var err error
		meta.Required, err = strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean for required: %s", value)
		}
	case "complete":
		meta.Complete = value
	case "args":
		args, err := parseArgumentSpecs(value)
		if err != nil {
			return fmt.Errorf("invalid args specification: %w", err)
		}
		meta.Args = args
	case "suffix":
		meta.Suffix = value
	case "enum":
		meta.Enum = parseEnumValues(value)
	default:
		return fmt.Errorf("unknown key in tag: %s", key)
	}
	
	return nil
}

func parseDefaultValue(value string, fieldType FieldType) interface{} {
	switch fieldType {
	case FieldTypeNumber:
		if num, err := strconv.ParseFloat(value, 64); err == nil {
			return num
		}
		return 0.0
	case FieldTypeFlag:
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
		return false
	default:
		return value
	}
}

// parseFlagName converts a field name to command line flag format
// Example: "InputFile" -> "-input-file", "X" -> "-x"
func parseFlagName(fieldName string) string {
	if len(fieldName) == 1 {
		return "-" + strings.ToLower(fieldName)
	}
	
	var result strings.Builder
	result.WriteString("-")
	
	for i, r := range fieldName {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteString("-")
		}
		if r >= 'A' && r <= 'Z' {
			result.WriteRune(r - 'A' + 'a')
		} else {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// parseArgumentSpecs parses the args specification for multi-argument switches
// Format: "field:content" or "field,pattern,replacement"
func parseArgumentSpecs(value string) ([]ArgumentSpec, error) {
	// Handle colon-separated format: "field:content"
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		specs := make([]ArgumentSpec, len(parts))
		
		for i, part := range parts {
			part = strings.TrimSpace(part)
			argType, err := parseArgumentType(part)
			if err != nil {
				return nil, err
			}
			specs[i] = ArgumentSpec{
				Name: part,
				Type: argType,
			}
		}
		return specs, nil
	}
	
	// Handle comma-separated format: "field,pattern,replacement"
	parts := strings.Split(value, ",")
	specs := make([]ArgumentSpec, len(parts))
	
	for i, part := range parts {
		part = strings.TrimSpace(part)
		argType, err := parseArgumentType(part)
		if err != nil {
			return nil, err
		}
		specs[i] = ArgumentSpec{
			Name: part,
			Type: argType,
		}
	}
	
	return specs, nil
}

// parseArgumentType converts a string to ArgumentType
func parseArgumentType(s string) (ArgumentType, error) {
	switch s {
	case "string":
		return ArgumentTypeString, nil
	case "field":
		return ArgumentTypeField, nil
	case "content":
		return ArgumentTypeContent, nil
	case "file":
		return ArgumentTypeFile, nil
	case "number":
		return ArgumentTypeNumber, nil
	default:
		// For compatibility, treat unknown types as string
		return ArgumentTypeString, nil
	}
}

// parseTagParts parses a tag string, respecting brace-enclosed values
// This handles cases like "file,global,last,suffix=.{tsv,csv}" where the value contains commas
func parseTagParts(tag string) []string {
	var parts []string
	var current strings.Builder
	braceDepth := 0
	
	for _, char := range tag {
		switch char {
		case '{':
			braceDepth++
			current.WriteRune(char)
		case '}':
			braceDepth--
			current.WriteRune(char)
		case ',':
			if braceDepth == 0 {
				// We're not inside braces, so this comma is a separator
				parts = append(parts, current.String())
				current.Reset()
			} else {
				// We're inside braces, so this comma is part of the value
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}
	
	// Add the final part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	
	return parts
}

// parseEnumValues parses enum values from a tag string
// Supports formats: "bar,line,area" or "bar:line:area"
func parseEnumValues(value string) []string {
	// Support both comma and colon as separators
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		for i, part := range parts {
			parts[i] = strings.TrimSpace(part)
		}
		return parts
	}
	
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		for i, part := range parts {
			parts[i] = strings.TrimSpace(part)
		}
		return parts
	}
	
	// Single value
	return []string{strings.TrimSpace(value)}
}