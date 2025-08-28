package gs

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// GSCommand represents a command with GS-style argument processing
type GSCommand struct {
	config      interface{} // Pointer to command configuration struct
	fields      []FieldMeta // Metadata for all fields
	completer   Completer   // Completion handler
	generator   DocumentGenerator // Documentation generator
	fieldCache  map[string][]string // TSV field name cache
	contentCache map[string]map[string][]string // TSV content cache: filename -> field -> values
	scanDepth   int // Number of lines to scan for content completion
}

// NewCommand creates a new GSCommand from a configuration struct
func NewCommand(config interface{}) (*GSCommand, error) {
	fields, err := reflectFields(config)
	if err != nil {
		return nil, fmt.Errorf("reflecting fields: %w", err)
	}
	
	cmd := &GSCommand{
		config:       config,
		fields:       fields,
		fieldCache:   make(map[string][]string),
		contentCache: make(map[string]map[string][]string),
		scanDepth:    100, // Default scan depth like TSVSelect
	}
	
	return cmd, nil
}

// Parse parses command line arguments into clauses
func (cmd *GSCommand) Parse(args []string) ([]ClauseSet, error) {
	clauses := []ClauseSet{}
	current := ClauseSet{
		Fields: make(map[string]interface{}),
	}
	global := make(map[string]interface{}) // Track global fields separately
	
	i := 0
	for i < len(args) {
		arg := args[i]
		
		switch {
		case arg == "+":
			// Start new negated clause (+ means negated for consistency)
			clauses = append(clauses, current)
			current = ClauseSet{
				Fields:    make(map[string]interface{}),
				IsNegated: true,
			}
			i++
			
		case arg == "-":
			// Start new positive clause (- means positive/normal)
			clauses = append(clauses, current)
			current = ClauseSet{
				Fields: make(map[string]interface{}),
			}
			i++
			
		case strings.HasPrefix(arg, "+"):
			// Handle +flag syntax (negated flag within current clause)
			if len(arg) > 1 {
				// Parse the flag part (remove the + prefix) but mark as negated
				flagArg := "-" + arg[1:] // Convert +flag to -flag
				consumed, err := cmd.parseFlagWithNegation(append([]string{flagArg}, args[i+1:]...), &current, global, true)
				if err != nil {
					return nil, err
				}
				i += consumed
			} else {
				// Just a standalone +, treat as negated clause separator
				clauses = append(clauses, current)
				current = ClauseSet{
					Fields:    make(map[string]interface{}),
					IsNegated: true,
				}
				i++
			}
			
		case strings.HasPrefix(arg, "-"):
			// Handle both -flag (positive) and explicit -switch syntax
			if len(arg) > 1 {
				// Regular -flag (positive)
				consumed, err := cmd.parseFlagWithNegation(args[i:], &current, global, false)
				if err != nil {
					return nil, err
				}
				i += consumed
			} else {
				// Just a standalone -, treat as positive clause separator (handled above)
				// This case should be caught by the standalone "-" case above
				clauses = append(clauses, current)
				current = ClauseSet{
					Fields: make(map[string]interface{}),
				}
				i++
			}
			
		default:
			// Positional argument (likely filename)
			current.Fields["_args"] = append(
				getStringSlice(current.Fields["_args"]), arg)
			
			// If this looks like a TSV file and no -argv has been set, treat as file input
			if strings.HasSuffix(strings.ToLower(arg), ".tsv") || strings.HasSuffix(strings.ToLower(arg), ".csv") {
				if _, hasArgv := current.Fields["Argv"]; !hasArgv {
					// Also check global fields to avoid overriding explicit -argv
					if _, hasGlobalArgv := global["Argv"]; !hasGlobalArgv {
						current.Fields["Argv"] = arg
					}
				}
			}
			i++
		}
	}
	
	// Add final clause
	clauses = append(clauses, current)
	
	// Apply global fields to all clauses
	for i := range clauses {
		for k, v := range global {
			if _, exists := clauses[i].Fields[k]; !exists {
				clauses[i].Fields[k] = v
			}
		}
	}
	
	// Apply defaults and validate
	if err := cmd.applyDefaults(clauses); err != nil {
		return nil, err
	}
	
	// Apply defaults to global fields too
	for _, fieldMeta := range cmd.fields {
		if fieldMeta.Scope == ScopeGlobal && fieldMeta.DefaultValue != nil {
			if _, exists := global[fieldMeta.Name]; !exists {
				global[fieldMeta.Name] = fieldMeta.DefaultValue
			}
		}
	}
	
	// Also apply global values to the config struct
	if err := cmd.applyGlobalToConfig(global); err != nil {
		return nil, err
	}
	
	return clauses, nil
}

// parseFlag parses a single flag and its value(s)
func (cmd *GSCommand) parseFlag(args []string, clause *ClauseSet, global map[string]interface{}) (int, error) {
	return cmd.parseFlagWithNegation(args, clause, global, false)
}

// parseFlagWithNegation parses a single flag and its value(s) with optional negation
func (cmd *GSCommand) parseFlagWithNegation(args []string, clause *ClauseSet, global map[string]interface{}, negated bool) (int, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("no arguments to parse")
	}
	
	flagName := args[0]
	
	// Find matching field
	var fieldMeta *FieldMeta
	for i := range cmd.fields {
		expected := parseFlagName(cmd.fields[i].Name)
		if flagName == expected {
			fieldMeta = &cmd.fields[i]
			break
		}
	}
	
	if fieldMeta == nil {
		return 0, fmt.Errorf("unknown flag: %s", flagName)
	}
	
	// Determine where to store the value based on scope
	var target map[string]interface{}
	if fieldMeta.Scope == ScopeGlobal {
		target = global
	} else {
		target = clause.Fields
	}
	
	// Handle flag types
	switch fieldMeta.Type {
	case FieldTypeFlag:
		// Boolean flag, no value needed
		target[fieldMeta.Name] = true
		return 1, nil
		
	case FieldTypeMulti:
		// Multi-argument switch
		if len(fieldMeta.Args) == 0 {
			return 0, fmt.Errorf("multi-argument flag %s has no argument specification", flagName)
		}
		
		requiredArgs := len(fieldMeta.Args)
		if len(args) < requiredArgs+1 {
			return 0, fmt.Errorf("flag %s requires %d arguments", flagName, requiredArgs)
		}
		
		// Parse each argument according to its specification
		argValues := make(map[string]interface{})
		for i, argSpec := range fieldMeta.Args {
			argValue := args[i+1]
			parsedValue, err := cmd.parseValueByArgumentType(argValue, argSpec.Type)
			if err != nil {
				return 0, fmt.Errorf("parsing argument %s for %s: %w", argSpec.Name, flagName, err)
			}
			argValues[argSpec.Name] = parsedValue
		}
		
		// Add negation information if the switch was negated
		if negated {
			argValues["_negated"] = true
		}
		
		// Handle list vs last mode for multi-argument switches
		if fieldMeta.Mode == ModeList {
			existing := target[fieldMeta.Name]
			if existing == nil {
				target[fieldMeta.Name] = []interface{}{argValues}
			} else {
				if list, ok := existing.([]interface{}); ok {
					target[fieldMeta.Name] = append(list, argValues)
				} else {
					target[fieldMeta.Name] = []interface{}{existing, argValues}
				}
			}
		} else {
			target[fieldMeta.Name] = argValues
		}
		
		return requiredArgs + 1, nil
		
	default:
		// Single-argument flag
		if len(args) < 2 {
			return 0, fmt.Errorf("flag %s requires a value", flagName)
		}
		
		value := args[1]
		parsedValue, err := cmd.parseValueWithValidation(value, fieldMeta)
		if err != nil {
			return 0, fmt.Errorf("parsing value for %s: %w", flagName, err)
		}
		
		// For single-argument switches, wrap in map if negated
		var valueToStore interface{} = parsedValue
		if negated {
			valueToStore = map[string]interface{}{
				"value":    parsedValue,
				"_negated": true,
			}
		}
		
		// Handle list vs last mode
		if fieldMeta.Mode == ModeList {
			existing := target[fieldMeta.Name]
			if existing == nil {
				target[fieldMeta.Name] = []interface{}{valueToStore}
			} else {
				if list, ok := existing.([]interface{}); ok {
					target[fieldMeta.Name] = append(list, valueToStore)
				} else {
					target[fieldMeta.Name] = []interface{}{existing, valueToStore}
				}
			}
		} else {
			target[fieldMeta.Name] = valueToStore
		}
		
		return 2, nil
	}
}

// parseValue converts a string value to the appropriate type
func (cmd *GSCommand) parseValue(value string, fieldType FieldType) (interface{}, error) {
	switch fieldType {
	case FieldTypeString, FieldTypeField, FieldTypeFile:
		return value, nil
	case FieldTypeNumber:
		return strconv.ParseFloat(value, 64)
	case FieldTypeFlag:
		return strconv.ParseBool(value)
	default:
		return value, nil
	}
}

// parseValueWithValidation converts a string value to the appropriate type and validates enum constraints
func (cmd *GSCommand) parseValueWithValidation(value string, fieldMeta *FieldMeta) (interface{}, error) {
	// First parse the value according to its type
	parsedValue, err := cmd.parseValue(value, fieldMeta.Type)
	if err != nil {
		return nil, err
	}
	
	// For string fields, check enum constraints
	if fieldMeta.Type == FieldTypeString && len(fieldMeta.Enum) > 0 {
		// Check if the value is in the allowed enum values
		found := false
		for _, enumValue := range fieldMeta.Enum {
			if value == enumValue {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid value '%s', must be one of: %s", 
				value, strings.Join(fieldMeta.Enum, ", "))
		}
	}
	
	return parsedValue, nil
}

// parseValueByArgumentType converts a string value based on ArgumentType
func (cmd *GSCommand) parseValueByArgumentType(value string, argType ArgumentType) (interface{}, error) {
	switch argType {
	case ArgumentTypeString, ArgumentTypeField, ArgumentTypeContent, ArgumentTypeFile:
		return value, nil
	case ArgumentTypeNumber:
		return strconv.ParseFloat(value, 64)
	default:
		return value, nil
	}
}

// applyDefaults applies default values to fields that weren't specified
func (cmd *GSCommand) applyDefaults(clauses []ClauseSet) error {
	for i := range clauses {
		for _, fieldMeta := range cmd.fields {
			if _, exists := clauses[i].Fields[fieldMeta.Name]; !exists {
				if fieldMeta.DefaultValue != nil {
					clauses[i].Fields[fieldMeta.Name] = fieldMeta.DefaultValue
				}
			}
		}
	}
	return nil
}

// applyGlobalToConfig applies global field values to the config struct using reflection
func (cmd *GSCommand) applyGlobalToConfig(global map[string]interface{}) error {
	if len(global) == 0 {
		return nil
	}
	
	configValue := reflect.ValueOf(cmd.config)
	if configValue.Kind() == reflect.Ptr {
		configValue = configValue.Elem()
	}
	
	for fieldName, value := range global {
		field := configValue.FieldByName(fieldName)
		if !field.IsValid() || !field.CanSet() {
			continue
		}
		
		valueReflect := reflect.ValueOf(value)
		if field.Type().AssignableTo(valueReflect.Type()) {
			field.Set(valueReflect)
		} else if field.Kind() == reflect.String && valueReflect.Kind() == reflect.String {
			field.SetString(valueReflect.String())
		} else if field.Kind() == reflect.Float64 && valueReflect.Kind() == reflect.Float64 {
			field.SetFloat(valueReflect.Float())
		} else if field.Kind() == reflect.Bool && valueReflect.Kind() == reflect.Bool {
			field.SetBool(valueReflect.Bool())
		}
	}
	
	return nil
}

// Execute runs the command with the given arguments
func (cmd *GSCommand) Execute(ctx context.Context, args []string) error {
	// Check for special flags first
	if len(args) > 0 {
		switch args[0] {
		case "-help", "--help":
			fmt.Println(cmd.GenerateHelp())
			return nil
		case "-man":
			fmt.Println(cmd.GenerateManPage())
			return nil
		case "-complete":
			return cmd.handleCompletion(args)
		case "-bash-completion":
			fmt.Print(cmd.generateBashCompletion())
			return nil
		}
	}
	
	clauses, err := cmd.Parse(args)
	if err != nil {
		return fmt.Errorf("parsing arguments: %w", err)
	}
	
	// Execute command if it implements Commander
	if commander, ok := cmd.config.(Commander); ok {
		// Validate configuration before execution
		if err := commander.Validate(); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		
		return commander.Execute(ctx, clauses)
	}
	
	return fmt.Errorf("command does not implement Commander interface")
}

// handleCompletion handles bash completion
func (cmd *GSCommand) handleCompletion(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("completion requires position and arguments")
	}
	
	pos, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid completion position: %s", args[1])
	}
	
	compArgs := args[2:]
	
	// pos is the position in the user's command line, use it as-is for compArgs
	// No adjustment needed - position semantics should be consistent
	
	// Use integrated completion logic
	completions, err := cmd.complete(compArgs, pos)
	if err != nil {
		return err
	}
	
	for _, completion := range completions {
		fmt.Println(completion)
	}
	
	return nil
}

// getStringSlice safely converts interface{} to []string
func getStringSlice(v interface{}) []string {
	if v == nil {
		return []string{}
	}
	if slice, ok := v.([]string); ok {
		return slice
	}
	return []string{}
}

// GenerateHelp generates help text (placeholder implementation)
func (cmd *GSCommand) GenerateHelp() string {
	var sb strings.Builder
	sb.WriteString("Usage: command [options]\n\nOptions:\n")
	
	for _, field := range cmd.fields {
		flag := parseFlagName(field.Name)
		sb.WriteString(fmt.Sprintf("  %-15s %s\n", flag, field.Help))
	}
	
	return sb.String()
}

// GenerateManPage generates a man page (placeholder implementation)
func (cmd *GSCommand) GenerateManPage() string {
	return "Man page not implemented yet"
}

// SetCompleter sets the completion handler
func (cmd *GSCommand) SetCompleter(completer Completer) {
	cmd.completer = completer
}

// GetFields returns the field metadata
func (cmd *GSCommand) GetFields() []FieldMeta {
	return cmd.fields
}

// generateBashCompletion generates a bash completion script
func (cmd *GSCommand) generateBashCompletion() string {
	// Extract command name from the program name
	// This would typically be set from os.Args[0] in a real implementation
	commandName := "chart" // Default for our example
	
	return fmt.Sprintf(`# Bash completion for %s
_%s_completion() {
    local cur prev words cword
    
    # Basic completion setup without _init_completion dependency
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    
    # Call the command with -complete to get all suggestions
    # The Go binary now handles all completion logic internally
    local completions
    completions=$(%s -complete $((COMP_CWORD-1)) "${COMP_WORDS[@]:1}" 2>/dev/null)
    
    # Generate completion replies from command output
    if [[ -n "$completions" ]]; then
        COMPREPLY=($(compgen -W "$completions" -- "$cur"))
    fi
}

# Register the completion function
complete -F _%s_completion %s
`, commandName, commandName, commandName, commandName, commandName)
}

// getFlagNames returns a space-separated list of all flag names
func (cmd *GSCommand) getFlagNames() string {
	var flags []string
	for _, field := range cmd.fields {
		flag := parseFlagName(field.Name)
		flags = append(flags, flag)  // Add -flag
		flags = append(flags, "+"+flag[1:])  // Add +flag (remove - and add +)
	}
	// Add common flags (these don't typically have + versions)
	flags = append(flags, "-help", "-man", "-complete", "-bash-completion")
	return strings.Join(flags, " ")
}

// CompletionContext represents the context for command completion
type CompletionContext struct {
	Type          CompletionType // What kind of completion this is
	Current       string         // Current word being completed
	TSVFile       string         // TSV file found in arguments
	FieldName     string         // Field name for content completion
	ArgumentIndex int            // Which argument of a multi-arg switch
	ArgumentSpec  *ArgumentSpec  // Specification for current argument
	FieldMeta     *FieldMeta     // Metadata for the current field (for suffix filtering)
}

// CompletionType represents different types of completion
type CompletionType int

const (
	CompletionFlag CompletionType = iota
	CompletionField
	CompletionContent
	CompletionFile
	CompletionMultiArg
	CompletionEnum
)

// complete provides completion for command line arguments
func (cmd *GSCommand) complete(args []string, pos int) ([]string, error) {
	context := cmd.analyzeCompletionContext(args, pos)
	
	switch context.Type {
	case CompletionFlag:
		return cmd.completeFlags(context.Current), nil
	case CompletionField:
		return cmd.completeField(context.TSVFile, context.Current)
	case CompletionContent:
		return cmd.completeContent(context.TSVFile, context.FieldName, context.Current)
	case CompletionMultiArg:
		return cmd.completeMultiArgument(context)
	case CompletionEnum:
		return cmd.completeEnum(context.FieldMeta, context.Current), nil
	case CompletionFile:
		return cmd.completeFilesWithSuffix(context.Current, context.FieldMeta)
	default:
		return cmd.completeFilesWithSuffix(context.Current, nil)
	}
}

// isFieldFlag checks if a flag expects a field name
func (cmd *GSCommand) isFieldFlag(flagName string) bool {
	for _, field := range cmd.fields {
		if parseFlagName(field.Name) == flagName && field.Type == FieldTypeField {
			return true
		}
	}
	return false
}

// findTSVFile searches for TSV files in command arguments
func (cmd *GSCommand) findTSVFile(args []string) string {
	for i, arg := range args {
		// Case 1: TSV/CSV file after flags like -argv
		if i > 0 && (args[i-1] == "-argv" || strings.HasSuffix(args[i-1], "-file")) {
			if strings.HasSuffix(arg, ".tsv") || strings.HasSuffix(arg, ".csv") {
				return arg
			}
		}
		
		// Case 2: Direct TSV/CSV file argument (bare argument, not following a flag)
		if (strings.HasSuffix(arg, ".tsv") || strings.HasSuffix(arg, ".csv")) && !strings.HasPrefix(arg, "-") {
			// Make sure it's not immediately after a flag that takes a value (exclude -argv case handled above)
			if i == 0 || !strings.HasPrefix(args[i-1], "-") || args[i-1] == "-" || args[i-1] == "+" {
				return arg // This is a positional argument
			}
		}
	}
	
	return ""
}

// completeField provides field name completion for a TSV file
func (cmd *GSCommand) completeField(filename, partial string) ([]string, error) {
	fields, err := cmd.getFields(filename)
	if err != nil {
		return nil, err
	}
	
	var matches []string
	partial = strings.ToLower(partial)
	
	for _, field := range fields {
		if strings.HasPrefix(strings.ToLower(field), partial) {
			matches = append(matches, field)
		}
	}
	
	return matches, nil
}

// getFields reads and caches field names from a TSV file
func (cmd *GSCommand) getFields(filename string) ([]string, error) {
	// Check cache first
	if fields, exists := cmd.fieldCache[filename]; exists {
		return fields, nil
	}
	
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", filename, err)
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, fmt.Errorf("file %s is empty", filename)
	}
	
	headerLine := scanner.Text()
	fields := cmd.parseTSVHeader(headerLine)
	
	// Cache the result
	cmd.fieldCache[filename] = fields
	
	return fields, nil
}

// parseTSVHeader parses a TSV header line into field names
func (cmd *GSCommand) parseTSVHeader(header string) []string {
	// Remove leading non-alphanumeric characters (common in TSV files)
	header = strings.TrimLeftFunc(header, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})
	
	// Split on tabs first, then commas as fallback
	var fields []string
	if strings.Contains(header, "\t") {
		fields = strings.Split(header, "\t")
	} else {
		fields = strings.Split(header, ",")
	}
	
	// Clean up field names
	var cleanFields []string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			cleanFields = append(cleanFields, field)
		}
	}
	
	return cleanFields
}

// analyzeCompletionContext analyzes the command line to determine completion context
func (cmd *GSCommand) analyzeCompletionContext(args []string, pos int) CompletionContext {
	context := CompletionContext{
		Type: CompletionFile, // Default fallback
	}
	
	// Get current word being completed
	if pos < len(args) {
		context.Current = args[pos]
	}
	
	// Check if completing a flag (both - and + prefixes)
	if strings.HasPrefix(context.Current, "-") || strings.HasPrefix(context.Current, "+") {
		context.Type = CompletionFlag
		return context
	}
	
	// Find TSV file for field/content completion
	context.TSVFile = cmd.findTSVFile(args)
	
	// Analyze backwards to find the flag that might need completion
	flagPos, fieldMeta := cmd.findLastFlag(args, pos)
	if fieldMeta == nil {
		// No flag found, this might be a bare file argument
		// Check if we should apply -argv completion behavior
		if cmd.shouldUseBareFileCompletion(args, pos) {
			// Find the Argv field metadata to use its suffix pattern
			for i := range cmd.fields {
				if cmd.fields[i].Name == "Argv" {
					context.FieldMeta = &cmd.fields[i]
					context.Type = CompletionFile
					return context
				}
			}
		}
		context.Type = CompletionFile
		return context
	}
	
	// Calculate which argument of the flag we're completing
	argIndex := pos - flagPos - 1
	
	// Set field metadata for suffix filtering
	context.FieldMeta = fieldMeta
	
	switch fieldMeta.Type {
	case FieldTypeField:
		// Simple field flag
		if argIndex == 0 {
			context.Type = CompletionField
		} else {
			context.Type = CompletionFile // Beyond expected arguments
		}
		
	case FieldTypeString:
		// String field - check if it has enum values
		if argIndex == 0 {
			if len(fieldMeta.Enum) > 0 {
				context.Type = CompletionEnum
			} else {
				context.Type = CompletionFile // No enum, default to file completion
			}
		} else {
			context.Type = CompletionFile // Beyond expected arguments
		}
		
	case FieldTypeMulti:
		// Multi-argument flag
		if argIndex >= 0 && argIndex < len(fieldMeta.Args) {
			context.Type = CompletionMultiArg
			context.ArgumentIndex = argIndex
			context.ArgumentSpec = &fieldMeta.Args[argIndex]
			
			// Set field name if we're completing content after a field argument
			if argIndex > 0 && fieldMeta.Args[argIndex-1].Type == ArgumentTypeField {
				if flagPos+argIndex < len(args) {
					context.FieldName = args[flagPos+argIndex]
				}
			}
		} else {
			context.Type = CompletionFile // Beyond expected arguments
		}
		
	default:
		// Other flag types that take single arguments
		if argIndex == 0 {
			context.Type = CompletionFile // Let default file completion handle it
		} else {
			context.Type = CompletionFile
		}
	}
	
	return context
}

// findLastFlag finds the most recent flag before the current position
func (cmd *GSCommand) findLastFlag(args []string, pos int) (int, *FieldMeta) {
	// Ensure we don't go beyond the bounds
	maxIndex := len(args) - 1
	if pos > maxIndex {
		pos = maxIndex + 1
	}
	
	for i := pos - 1; i >= 0; i-- {
		if i >= len(args) {
			continue
		}
		if strings.HasPrefix(args[i], "-") || strings.HasPrefix(args[i], "+") {
			// Found a potential flag, normalize it
			flagArg := args[i]
			var normalizedFlag string
			
			if strings.HasPrefix(flagArg, "+") && len(flagArg) > 1 {
				// Convert +switch to -switch for matching
				normalizedFlag = "-" + flagArg[1:]
			} else if strings.HasPrefix(flagArg, "-") && len(flagArg) > 1 {
				// Already normalized
				normalizedFlag = flagArg
			} else {
				// Standalone + or -, not a flag
				return -1, nil
			}
			
			// Check if it's a valid flag
			for j := range cmd.fields {
				expected := parseFlagName(cmd.fields[j].Name)
				if normalizedFlag == expected {
					return i, &cmd.fields[j]
				}
			}
			// If we found a flag but it's not ours, stop looking
			// (this prevents going past clause boundaries)
			return -1, nil
		}
	}
	return -1, nil
}

// completeMultiArgument handles completion for multi-argument switches
func (cmd *GSCommand) completeMultiArgument(context CompletionContext) ([]string, error) {
	if context.ArgumentSpec == nil {
		return cmd.completeFiles(context.Current)
	}
	
	switch context.ArgumentSpec.Type {
	case ArgumentTypeField:
		if context.TSVFile != "" {
			return cmd.completeField(context.TSVFile, context.Current)
		}
		return []string{}, nil
		
	case ArgumentTypeContent:
		if context.TSVFile != "" && context.FieldName != "" {
			return cmd.completeContent(context.TSVFile, context.FieldName, context.Current)
		}
		return []string{}, nil
		
	case ArgumentTypeFile:
		return cmd.completeFiles(context.Current)
		
	default:
		// For string, number, etc. - no specific completion
		return []string{}, nil
	}
}

// completeContent provides completion for field content
func (cmd *GSCommand) completeContent(filename, fieldName, partial string) ([]string, error) {
	values, err := cmd.getFieldValues(filename, fieldName)
	if err != nil {
		return []string{}, nil // Return empty on error rather than failing
	}
	
	var matches []string
	partial = strings.ToLower(partial)
	
	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(value), partial) {
			matches = append(matches, value)
		}
	}
	
	return matches, nil
}

// getFieldValues scans TSV file and returns unique values for a specific field
func (cmd *GSCommand) getFieldValues(filename, fieldName string) ([]string, error) {
	// Check cache first
	if fileCache, exists := cmd.contentCache[filename]; exists {
		if values, exists := fileCache[fieldName]; exists {
			return values, nil
		}
	}
	
	// Initialize file cache if needed
	if _, exists := cmd.contentCache[filename]; !exists {
		cmd.contentCache[filename] = make(map[string][]string)
	}
	
	// Open file and parse content
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", filename, err)
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, fmt.Errorf("file %s is empty", filename)
	}
	
	// Parse header to get field positions
	headerLine := scanner.Text()
	fields := cmd.parseTSVHeader(headerLine)
	
	// Find the field index
	fieldIndex := -1
	for i, field := range fields {
		if field == fieldName {
			fieldIndex = i
			break
		}
	}
	
	if fieldIndex == -1 {
		return []string{}, nil // Field not found
	}
	
	// Scan content lines
	values := make(map[string]bool) // Use map to track unique values
	linesScanned := 0
	
	for scanner.Scan() && linesScanned < cmd.scanDepth {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		
		// Use comma fallback if not enough tab-separated parts
		if len(parts) <= fieldIndex {
			parts = strings.Split(line, ",")
		}
		
		if fieldIndex < len(parts) {
			value := strings.TrimSpace(parts[fieldIndex])
			if value != "" {
				values[value] = true
			}
		}
		
		linesScanned++
	}
	
	// Convert map to sorted slice
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	
	// Simple sort for consistent ordering
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	
	// Cache the result
	cmd.contentCache[filename][fieldName] = result
	
	return result, nil
}

// completeFlags provides flag name completion
func (cmd *GSCommand) completeFlags(partial string) []string {
	var matches []string
	partial = strings.ToLower(partial)
	
	// Add command-specific flags (both - and + versions)
	for _, field := range cmd.fields {
		flag := parseFlagName(field.Name)
		
		// Add -flag version
		if strings.HasPrefix(strings.ToLower(flag), partial) {
			matches = append(matches, flag)
		}
		
		// Add +flag version  
		plusFlag := "+" + flag[1:]  // Remove - and add +
		if strings.HasPrefix(strings.ToLower(plusFlag), partial) {
			matches = append(matches, plusFlag)
		}
	}
	
	// Add common flags (these don't typically have + versions)
	commonFlags := []string{"-help", "-man", "-complete", "-bash-completion"}
	for _, flag := range commonFlags {
		if strings.HasPrefix(strings.ToLower(flag), partial) {
			matches = append(matches, flag)
		}
	}
	
	return matches
}

// completeFiles provides file completion with preference for .tsv files
func (cmd *GSCommand) completeFiles(partial string) ([]string, error) {
	return cmd.completeFilesWithSuffix(partial, nil)
}

// completeFilesWithSuffix provides file completion with optional suffix filtering
func (cmd *GSCommand) completeFilesWithSuffix(partial string, fieldMeta *FieldMeta) ([]string, error) {
	// Get directory and filename pattern
	var dir string
	var pattern string
	
	if strings.HasSuffix(partial, "/") {
		// Path ends with /, complete contents of that directory
		dir = partial
		pattern = ""
	} else if strings.Contains(partial, "/") {
		// Path contains /, split into directory and pattern
		dir = filepath.Dir(partial)
		pattern = filepath.Base(partial)
	} else {
		// No path separator, complete in current directory
		dir = "."
		pattern = partial
	}
	
	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}, nil // Return empty on error rather than failing
	}
	
	var tsvFiles, otherFiles, directories []string
	
	for _, entry := range entries {
		name := entry.Name()
		
		// Skip hidden files unless explicitly requested
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(pattern, ".") {
			continue
		}
		
		// Check if name matches partial pattern
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(pattern)) {
			continue
		}
		
		// Build full path
		var fullPath string
		if dir == "." {
			fullPath = name
		} else {
			fullPath = filepath.Join(dir, name)
		}
		
		// Add trailing slash for directories
		if entry.IsDir() {
			fullPath += "/"
			// Always include directories regardless of suffix filtering
			directories = append(directories, fullPath)
			continue
		}
		
		// Apply suffix filtering if specified
		if fieldMeta != nil && fieldMeta.Suffix != "" {
			if !matchesSuffixPattern(name, fieldMeta.Suffix) {
				continue // Skip files that don't match the required suffix pattern
			}
		}
		
		// Prioritize TSV files (if no specific suffix required, or if suffix is .tsv)
		if strings.HasSuffix(strings.ToLower(name), ".tsv") {
			tsvFiles = append(tsvFiles, fullPath)
		} else {
			otherFiles = append(otherFiles, fullPath)
		}
	}
	
	// Return files first (TSV files, then other files), followed by directories
	result := append(tsvFiles, otherFiles...)
	result = append(result, directories...)
	return result, nil
}

// completeEnum provides completion for enumerated string values
func (cmd *GSCommand) completeEnum(fieldMeta *FieldMeta, partial string) []string {
	if fieldMeta == nil || len(fieldMeta.Enum) == 0 {
		return []string{}
	}
	
	var matches []string
	partial = strings.ToLower(partial)
	
	for _, enumValue := range fieldMeta.Enum {
		if strings.HasPrefix(strings.ToLower(enumValue), partial) {
			matches = append(matches, enumValue)
		}
	}
	
	return matches
}

// matchesSuffixPattern checks if a filename matches a suffix pattern
// Supports simple suffixes (.tsv), character classes (.[tc]sv), and brace expansion (.{tsv,csv})
func matchesSuffixPattern(filename, pattern string) bool {
	filename = strings.ToLower(filename)
	pattern = strings.ToLower(pattern)
	
	// Handle brace expansion patterns like .{tsv,csv}
	if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
		return matchesBracePattern(filename, pattern)
	}
	
	// If pattern contains glob characters, use filepath.Match
	if strings.ContainsAny(pattern, "*?[]") {
		// For suffix patterns, we need to match the end of the filename
		// Convert suffix pattern to full filename pattern
		fullPattern := "*" + pattern
		matched, err := filepath.Match(fullPattern, filename)
		if err != nil {
			// If pattern is invalid, fall back to simple suffix match
			return strings.HasSuffix(filename, pattern)
		}
		return matched
	}
	
	// Simple suffix matching for non-glob patterns
	return strings.HasSuffix(filename, pattern)
}

// matchesBracePattern handles brace expansion patterns like .{tsv,csv}
func matchesBracePattern(filename, pattern string) bool {
	// Find the brace section
	start := strings.Index(pattern, "{")
	end := strings.Index(pattern, "}")
	if start == -1 || end == -1 || start >= end {
		return false
	}
	
	prefix := pattern[:start]
	suffix := pattern[end+1:]
	options := strings.Split(pattern[start+1:end], ",")
	
	// Test each option
	for _, option := range options {
		testPattern := prefix + strings.TrimSpace(option) + suffix
		if strings.HasSuffix(filename, testPattern) {
			return true
		}
	}
	
	return false
}

// shouldUseBareFileCompletion determines if we should treat a bare argument as a file argument
// like -argv, applying the same suffix filtering
func (cmd *GSCommand) shouldUseBareFileCompletion(args []string, pos int) bool {
	// If we're at position 0, this is likely a bare file argument
	if pos == 0 {
		return true
	}
	
	// Check if the previous argument is not a flag or is a clause separator
	if pos > 0 && pos-1 < len(args) {
		prev := args[pos-1]
		if prev == "+" || prev == "-" || (!strings.HasPrefix(prev, "-") && !strings.HasPrefix(prev, "+")) {
			return true
		}
	}
	
	return false
}