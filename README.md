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
type TSV2ChartConfig struct {
    X         string                      `gs:"field,global,last,help=Use field for X axis"`
    Y         []string                    `gs:"field,local,list,help=Use field for Y axis"`
    Match     []map[string]interface{}    `gs:"multi,local,list,args=field:content,help=Filter by field matching content"`
    Right     bool                        `gs:"flag,local,last,help=Use right-hand scale"`
    Title     string                      `gs:"string,global,last,help=Chart title,default=Chart"`
    Type      string                      `gs:"string,global,last,help=Chart type: bar/line/area,default=bar,enum=bar:line:area"`
    Width     float64                     `gs:"number,global,last,help=Chart width in pixels,default=800"`
    Height    float64                     `gs:"number,global,last,help=Chart height in pixels,default=400"`
    Quiet     bool                        `gs:"flag,global,last,help=Suppress progress messages,default=true"`
    Argv      string                      `gs:"file,global,last,help=Input TSV file,suffix=.[tc]sv"`
}
```

### 2. Implement the Commander Interface

```go
func (cfg *TSV2ChartConfig) Execute(ctx context.Context, clauses []gs.ClauseSet) error {
    // Get input file (supports both -argv flag and bare arguments)
    inputFile := cfg.getInputFile(clauses)
    if inputFile == "" {
        return fmt.Errorf("no input file specified")
    }
    
    // Parse TSV data
    data, err := parseTSV(inputFile)
    if err != nil {
        return fmt.Errorf("parsing TSV file: %w", err)
    }
    
    // Process clauses and generate Chart.js HTML
    return cfg.generateChart(data, clauses)
}

func (cfg *TSV2ChartConfig) Validate() error {
    // Enum validation now handled during parsing
    return nil
}
```

### 3. Create and Execute the Command

```go
func main() {
    config := &TSV2ChartConfig{}
    
    cmd, err := gs.NewCommand(config)
    if err != nil {
        log.Fatal(err)
    }
    
    // TSV field completion is built into GSCommand
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
- `enum=bar:line:area` - Enumerated values for string field completion and validation

## Key Improvements

### Unix-Style File Arguments

GoGSTools now supports standard Unix command-line conventions with **bare file arguments**:

```bash
# Modern Unix-style syntax (preferred)
chart data.tsv -x time -y cpu_usage

# Traditional explicit syntax (still supported)  
chart -argv data.tsv -x time -y cpu_usage
```

**Benefits:**
- **Automatic detection**: `.tsv` and `.csv` files are automatically recognized as input files
- **Smart completion**: Bare file arguments get the same intelligent completion as `-argv`, including suffix filtering (`.[tc]sv`)
- **Backwards compatible**: Existing `-argv` syntax continues to work
- **Position flexible**: File can appear anywhere: `chart -x time data.tsv -y cpu`

### Parsing-Time Validation

Enum validation now happens **during command parsing** instead of after the fact, providing immediate feedback:

```bash
# Invalid enum value fails immediately with clear message
$ chart data.tsv -type pie
Error: parsing value for -type: invalid value 'pie', must be one of: bar, line, area

# Valid enum values work as expected  
$ chart data.tsv -type line
Chart Command Executed! Type: line
```

**Benefits:**
- **Immediate feedback**: Users see validation errors right away
- **Better error messages**: Clear indication of valid options
- **Single source of truth**: Enum constraints defined once in struct tags
- **Consistent with completion**: Same enum values power both validation and tab completion

## Clause-Based Logic

GoGSTools supports the same powerful clause system as the original TSVTools:

```bash
# Multiple conditions within a clause are ANDed together
tsv2chart data.tsv -y cpu_usage -y memory_usage

# Different clauses are ORed together  
tsv2chart data.tsv -y cpu_usage + -y disk_io -right
#                  ^^^^^^^^^^^^^   ^^^^^^^^^^^^^^^ 
#                  Clause 1        Clause 2

# This creates: (cpu_usage) OR (disk_io on right axis)
```

### Practical Examples

**Basic chart with two Y-axis fields (Unix-style syntax):**
```bash
# New preferred syntax - bare file arguments
tsv2chart data.tsv -x time -y cpu_usage -y memory_usage

# Traditional syntax still supported
tsv2chart -argv data.tsv -x time -y cpu_usage -y memory_usage
```

**Dual-axis chart with clause-based grouping:**
```bash
tsv2chart data.tsv -x time -y cpu_usage -y memory_usage + -y disk_io -right
```

**Different chart types:**
```bash
tsv2chart data.tsv -type line -title "Performance Over Time" -x time -y cpu_usage
```

**Multi-argument switches with content filtering:**
```bash
# Filter data where cpu_usage > 30 and memory_usage < 50
tsv2chart data.tsv -match cpu_usage 30 -match memory_usage 50

# Combine with clauses - different filters ORed together
tsv2chart data.tsv -match name Alice + -match department Engineering
```

**Universal switch negation:**
```bash
# Include all Y fields except cpu_usage
tsv2chart data.tsv -y memory_usage -y disk_io +y cpu_usage

# Mixed positive and negative switches
tsv2chart data.tsv -match active true +match archived true
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
chart data.tsv -match cpu_usage <TAB>    # Shows: 25.5 35.0 45.0 30.0 40.0
chart data.tsv -match name <TAB>         # Shows: Alice Bob Charlie David
```

### Universal Switch Negation
Any switch can be prefixed with `+` for negation or `-` for positive:

```bash
chart data.tsv -y cpu_usage      # Include cpu_usage field
chart data.tsv +y cpu_usage      # Exclude cpu_usage field (negated)
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
chart data.tsv -type <TAB>      # Shows: bar line area
chart data.tsv -type l<TAB>     # Completes to: line
```

### Directory Navigation
File completion supports directory traversal:

```bash
chart /tmp/<TAB>            # Shows files in /tmp matching suffix + subdirectories
chart data/<TAB>            # Shows files in data/ directory
```

## Bash Completion Setup

GoGSTools provides sophisticated bash completion that must be installed to work properly.

### Quick Installation

```bash
# 1. Build your command
cd examples/chart
go build -o tsv2chart

# 2. Install completion (user-specific)
mkdir -p ~/.local/share/bash-completion/completions
./tsv2chart -bash-completion > ~/.local/share/bash-completion/completions/tsv2chart

# 3. Restart shell or source completion
exec bash
```

### Alternative Installation Methods

**System-wide installation (requires sudo):**
```bash
sudo ./tsv2chart -bash-completion > /etc/bash_completion.d/tsv2chart
exec bash
```

**Session-only installation:**
```bash
eval "$(./tsv2chart -bash-completion)"
```

### Verification

Test that completion is working:
```bash
# Check completion is registered
complete -p tsv2chart

# Test completion manually  
tsv2chart -complete 2 data.tsv -type             # Should show: bar line area
tsv2chart -complete 1 data.tsv -y                # Should show TSV field names
```

### Troubleshooting

1. **Ensure bash-completion is installed:**
   ```bash
   # Ubuntu/Debian
   sudo apt install bash-completion
   
   # macOS
   brew install bash-completion
   ```

2. **Verify tsv2chart binary path:**
   ```bash
   which tsv2chart    # Should show the binary location
   ```

3. **Re-source completion:**
   ```bash
   source ~/.bashrc
   # OR restart terminal
   ```

## Auto-Documentation

Every command automatically supports help and documentation:

```bash
# Show help
tsv2chart -help

# Show man page  
tsv2chart -man

# Generate bash completion
tsv2chart -bash-completion
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
    └── chart/         # Complete TSV2Chart implementation
        ├── main.go    # Full-featured TSV-to-Chart.js processor
        └── tsv2chart  # Production-ready binary
```

## Installation

```bash
go get github.com/rosscartlidge/gogstools
```

## Real-World Example: System Process Visualization

Here's a practical example that creates an interactive chart showing process counts by user using system data:

```bash
# Collect process data by user from the system
echo -e "uid\tuser\tprocess_count" > process_count.tsv
ps -eo uid,user --no-headers | awk '{
    count[$1]++; users[$1]=$2
} END {
    for(uid in count) print uid"\t"users[uid]"\t"count[uid]
}' | sort -k3 -nr >> process_count.tsv

# Generate an interactive bar chart
tsv2chart process_count.tsv -type bar -x user -y process_count \
    -title "System Process Count by User" \
    -width 1000 -height 600 > process_chart.html

# Open the chart in your browser
open process_chart.html  # macOS
# or: xdg-open process_chart.html  # Linux
```

This creates a responsive HTML chart with:
- **Interactive tooltips** showing exact process counts
- **Deterministic colors** - same user always gets same color  
- **Professional styling** with Chart.js
- **Responsive design** that adapts to screen size

### Advanced System Analysis

```bash
# Create time-series data of CPU usage (requires multiple samples)
for i in {1..10}; do
    echo "$i\t$(cat /proc/loadavg | cut -d' ' -f1)" >> cpu_load.tsv
    sleep 2
done

# Add header and create line chart
sed -i '1i time\tload_avg' cpu_load.tsv
tsv2chart cpu_load.tsv -type line -x time -y load_avg \
    -title "System Load Average Over Time" > load_chart.html
```

## Examples

The `examples/tsv2chart` directory provides a complete working implementation that demonstrates:
- **Complete TSV processing**: Automatic separator detection (tab vs comma)
- **Clause-based stacking**: Multiple `-y` fields in same clause stack together  
- **Dual-axis support**: `-right` flag for right Y-axis scaling
- **Interactive Chart.js output**: Hover tooltips, responsive design
- **Field-aware completion**: Tab completion from actual TSV field names
- **Content completion**: Shows actual field values from data
- **Regex filtering**: `-match field pattern` for data filtering
- **Quiet mode**: Silent by default, verbose with `+quiet`

## Contributing

This project aims to stay as close as possible to Go idioms while preserving the power and elegance of the original TSVTools `gs_*` system. 

## License

MIT License - see LICENSE file for details.

## Acknowledgments

Inspired by the brilliant design of [TSVTools](https://github.com/csv2/tsv) by Ross Cartlidge. The original `gs_*` system demonstrated how sophisticated CLI interfaces could be built with declarative configuration and self-parsing completion.
