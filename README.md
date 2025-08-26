# GoGSTools - Go Implementation of TSVTools' gs_* System

GoGSTools is a Go package that brings the sophisticated command-line interface capabilities of [TSVTools' `gs_*` system](https://github.com/csv2/tsv) to Go applications. It provides self-documenting, completion-aware CLI tools with clause-based Boolean logic, just like the original TCL implementation.

## Features

🚀 **Struct Tag-Based Configuration** - Define your CLI using Go struct tags  
🔧 **Clause-Based Logic** - Support for complex Boolean operations with `+` and `-`  
📝 **Auto-Documentation** - Generate help and man pages from struct definitions  
⚡ **Field-Aware Completion** - TSV field name completion and context-sensitive suggestions  
🎯 **Type Safety** - Compile-time validation with Go's type system  
🔍 **Self-Parsing** - Tools generate their own bash completion  

## Quick Start

### 1. Define Your Command Structure

```go
type ChartConfig struct {
    X         string                      `gs:"field,global,last,help=Use field for X axis"`
    Y         []string                    `gs:"field,local,list,help=Use field for Y axis"`
    Match     []map[string]interface{}    `gs:"multi,local,list,args=field:content,help=Filter by field matching content"`
    Right     bool                        `gs:"flag,local,last,help=Use right-hand scale"`
    Title     string                      `gs:"string,global,last,help=Chart title,default=Chart"`
    Type      string                      `gs:"string,global,last,help=Chart type: bar/line/area,default=bar"`
    Argv      string                      `gs:"file,global,last,help=Input TSV file,suffix=.[tc]sv"`
    Config    string                      `gs:"file,global,last,help=Configuration file,suffix=.json"`
}
```

### 2. Implement the Commander Interface

```go
func (cfg *ChartConfig) Execute(ctx context.Context, clauses []gs.ClauseSet) error {
    fmt.Printf("Creating %s chart: %s\n", cfg.Type, cfg.Title)
    
    for i, clause := range clauses {
        fmt.Printf("Clause %d:\n", i+1)
        if yFields, ok := clause.Fields["Y"]; ok {
            fmt.Printf("  Y fields: %v\n", yFields)
        }
        if right, ok := clause.Fields["Right"]; ok && right.(bool) {
            fmt.Printf("  Using right axis\n")
        }
    }
    return nil
}

func (cfg *ChartConfig) Validate() error {
    validTypes := []string{"bar", "line", "area"}
    for _, valid := range validTypes {
        if cfg.Type == valid {
            return nil
        }
    }
    return fmt.Errorf("invalid chart type: %s", cfg.Type)
}
```

### 3. Create and Execute the Command

```go
func main() {
    config := &ChartConfig{}
    
    cmd, err := gs.NewCommand(config)
    if err != nil {
        log.Fatal(err)
    }
    
    // Optional: Add TSV field completion
    cmd.SetCompleter(completion.NewTSVCompleter())
    
    if err := cmd.Execute(context.Background(), os.Args[1:]); err != nil {
        log.Fatal(err)
    }
}
```

## Struct Tag Syntax

The `gs:` struct tag defines how each field behaves in the CLI:

```go
`gs:"type,scope,mode,key=value,key=value,..."`
```

### Field Types
- `string` - Text values
- `field` - TSV field names (enables completion)
- `file` - File paths (supports suffix filtering)
- `number` - Numeric values  
- `flag` - Boolean flags
- `multi` - Multi-argument switches (e.g., `-match field value`)

### Scopes
- `global` - Applies to entire command
- `local` - Applies within each clause

### Modes
- `last` - Keep only the last value
- `list` - Collect all values into a list

### Key-Value Options
- `help=...` - Help text for this field
- `default=...` - Default value
- `required=true` - Mark field as required
- `args=field:content` - Multi-argument switches (e.g., `-match field value`)
- `suffix=.tsv` - File completion filtering (supports glob patterns)
- `enum=bar:line:area` - Enumerated values for string field completion

## Clause-Based Logic

GoGSTools supports the same powerful clause system as the original TSVTools:

```bash
# Multiple conditions within a clause are ANDed together
chart -y cpu_usage -y memory_usage data.tsv

# Different clauses are ORed together  
chart -y cpu_usage + -y disk_io -right data.tsv
#     ^^^^^^^^^^^^^   ^^^^^^^^^^^^^^^ 
#     Clause 1        Clause 2

# This creates: (cpu_usage) OR (disk_io on right axis)
```

### Practical Examples

**Basic chart with two Y-axis fields:**
```bash
chart -x time -y cpu_usage -y memory_usage data.tsv
```

**Dual-axis chart with clause-based grouping:**
```bash
chart -x time -y cpu_usage -y memory_usage + -y disk_io -right data.tsv
```

**Different chart types:**
```bash
chart -type line -title "Performance Over Time" -x time -y cpu_usage data.tsv
```

**Multi-argument switches with content filtering:**
```bash
# Filter data where cpu_usage > 30 and memory_usage < 50
chart -match cpu_usage 30 -match memory_usage 50 data.tsv

# Combine with clauses - different filters ORed together
chart -match name Alice + -match department Engineering data.tsv
```

**Universal switch negation:**
```bash
# Include all Y fields except cpu_usage
chart -y memory_usage -y disk_io +y cpu_usage data.tsv

# Mixed positive and negative switches
chart -match active true +match archived true data.tsv
```

## Advanced Completion Features

GoGSTools provides sophisticated bash completion with multiple advanced features:

### Field-Aware Completion
When your struct contains `field` type fields and TSV files are specified, GoGSTools automatically provides field name completion:

```bash
# Tab completion will suggest: time, cpu_usage, memory_usage, disk_io, network_rx
chart -x <TAB> data.tsv
chart -y <TAB> data.tsv
```

### Content-Aware Completion
Multi-argument switches show actual field values from TSV data:

```bash
type ChartConfig struct {
    Match []map[string]interface{} `gs:"multi,local,list,args=field:content,help=Filter by field matching content"`
}

# Completion shows actual field names, then actual values from that field
chart -match cpu_usage <TAB>    # Shows: 25.5 35.0 45.0 30.0 40.0
chart -match name <TAB>         # Shows: Alice Bob Charlie David
```

### Universal Switch Negation
Any switch can be prefixed with `+` for negation or `-` for positive:

```bash
chart -y cpu_usage      # Include cpu_usage field
chart +y cpu_usage      # Exclude cpu_usage field (negated)
```

### File Completion with Glob Pattern Filtering
File fields support sophisticated suffix filtering:

```bash
type ChartConfig struct {
    Argv   string `gs:"file,global,last,suffix=.tsv"`           # Only .tsv files
    Data   string `gs:"file,global,last,suffix=.[tc]sv"`        # .tsv or .csv files (character class)
    Input  string `gs:"file,global,last,suffix=.{tsv,csv}"`     # .tsv or .csv files (brace expansion)
    Config string `gs:"file,global,last,suffix=*.json"`        # .json files with wildcard
}

# Tab completion filtered by suffix, plus directories for navigation
chart -argv <TAB>       # Shows only .tsv files + directories
chart -data <TAB>       # Shows .tsv and .csv files + directories
```

### Enumerated String Completion
String fields with predefined values show completion options:

```bash
type ChartConfig struct {
    Type string `gs:"string,global,last,enum=bar:line:area,default=bar"`
}

# Tab completion shows available enum values
chart -type <TAB>      # Shows: bar line area
chart -type l<TAB>     # Completes to: line
```

### Directory Navigation
File completion supports directory traversal:

```bash
chart -argv /tmp/<TAB>      # Shows files in /tmp matching suffix + subdirectories
chart -argv data/<TAB>      # Shows files in data/ directory
```

## Auto-Documentation

Every command automatically supports help and documentation:

```bash
# Show help
chart -help

# Show man page  
chart -man

# Show usage
chart -usage
```

## Comparison with Original TSVTools

| Feature | TSVTools (TCL) | GoGSTools |
|---------|---------------|-----------|
| Struct Tags | ❌ | ✅ |
| Clause Logic | ✅ | ✅ |
| Field Completion | ✅ | ✅ |
| Content-Aware Completion | ✅ | ✅ |
| Multi-Argument Switches | ✅ | ✅ |
| Universal Switch Negation | ❌ | ✅ |
| Glob Pattern File Filtering | ❌ | ✅ |
| Auto-Documentation | ✅ | ✅ |
| Type Safety | ❌ | ✅ |
| Compilation | Interpreted | Compiled |
| Performance | Good | Excellent |
| Memory Usage | Higher | Lower |

## Package Structure

```
github.com/rosscartlidge/gogstools/
├── gs/                 # Core command processing
│   ├── types.go       # Type definitions and interfaces  
│   ├── parser.go      # Struct tag parsing
│   ├── command.go     # Main command execution with integrated completion
│   └── command_test.go # Comprehensive test suite
└── examples/           # Example implementations
    └── chart/         # Working chart command demonstrator
```

## Installation

```bash
go get github.com/rosscartlidge/gogstools
```

## Examples

See the `examples/chart` directory for a complete working example that demonstrates:
- Multi-field Y-axis support with clause-based stacking
- Global vs local field scopes  
- Automatic field completion from TSV files
- Type conversion and validation
- Help generation

## Contributing

This project aims to stay as close as possible to Go idioms while preserving the power and elegance of the original TSVTools `gs_*` system. 

## License

MIT License - see LICENSE file for details.

## Acknowledgments

Inspired by the brilliant design of [TSVTools](https://github.com/csv2/tsv) by Ross Cartlidge. The original `gs_*` system demonstrated how sophisticated CLI interfaces could be built with declarative configuration and self-parsing completion.