package completion

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// TSVCompleter provides field completion for TSV files
type TSVCompleter struct {
	cache map[string][]string // filename -> field names cache
}

// NewTSVCompleter creates a new TSV completer
func NewTSVCompleter() *TSVCompleter {
	return &TSVCompleter{
		cache: make(map[string][]string),
	}
}

// Complete provides completion for command line arguments
func (tc *TSVCompleter) Complete(ctx context.Context, args []string, pos int) ([]string, error) {
	var current string
	if pos < len(args) {
		current = args[pos]
	}
	// If pos >= len(args), current remains empty string (completing after cursor)
	
	// Look for the previous argument to determine context
	var prevArg string
	if pos > 0 {
		if pos-1 < len(args) {
			prevArg = args[pos-1]
		} else if len(args) > 0 {
			// We're completing after the last argument
			prevArg = args[len(args)-1]
		}
	}
	
	// Check if we're completing a field argument (-x, -y, etc.)
	isFieldFlag := (prevArg == "-x" || prevArg == "-y" || strings.Contains(prevArg, "field"))
	
	// If we're completing a field, find TSV files in the command line
	if isFieldFlag || (!strings.HasPrefix(current, "-") && current != "") {
		// Look for TSV files in all arguments, including after flags like -argv
		tsvFile := tc.findTSVFile(args)
		if tsvFile != "" {
			fields, err := tc.CompleteField(tsvFile, current)
			if err == nil && len(fields) > 0 {
				return fields, nil
			}
		}
		
		// If no TSV file found, try the current argument as a filename
		if strings.HasSuffix(current, ".tsv") {
			fields, err := tc.CompleteField(current, "")
			if err == nil {
				return fields, nil
			}
		}
	}
	
	return []string{}, nil
}

// findTSVFile searches for TSV files in command arguments, handling both
// positional arguments and flag-value pairs like -argv filename.tsv
func (tc *TSVCompleter) findTSVFile(args []string) string {
	for i, arg := range args {
		// Case 1: Direct TSV file argument (not following a flag)
		if strings.HasSuffix(arg, ".tsv") && !strings.HasPrefix(arg, "-") {
			// Make sure it's not immediately after a flag that takes a value
			if i > 0 && (args[i-1] == "-argv" || strings.HasSuffix(args[i-1], "-file")) {
				return arg // This is a file after a file flag
			} else if i == 0 || !strings.HasPrefix(args[i-1], "-") {
				return arg // This is a positional argument
			}
		}
		
		// Case 2: TSV file after flags like -argv
		if i > 0 && (args[i-1] == "-argv" || strings.HasSuffix(args[i-1], "-file")) && strings.HasSuffix(arg, ".tsv") {
			return arg
		}
	}
	
	return ""
}

// CompleteField provides field name completion for a TSV file
func (tc *TSVCompleter) CompleteField(filename, partial string) ([]string, error) {
	fields, err := tc.getFields(filename)
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
func (tc *TSVCompleter) getFields(filename string) ([]string, error) {
	// Check cache first
	if fields, exists := tc.cache[filename]; exists {
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
	fields := parseTSVHeader(headerLine)
	
	// Cache the result
	tc.cache[filename] = fields
	
	return fields, nil
}

// parseTSVHeader parses a TSV header line into field names
func parseTSVHeader(header string) []string {
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