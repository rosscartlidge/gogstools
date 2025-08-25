# Bash Completion Setup for GoGSTools

This guide shows how to set up bash completion for the GoGSTools chart command.

## Quick Setup

### 1. Generate the completion script
```bash
# Build the chart binary
go build -o chart ./examples/chart

# Generate completion script
./chart -bash-completion > chart_completion.sh
```

### 2. Install completion (choose one method)

#### Method A: User-specific installation
```bash
# Create completion directory
mkdir -p ~/.local/share/bash-completion/completions

# Install completion
./chart -bash-completion > ~/.local/share/bash-completion/completions/chart

# Restart your shell or source bashrc
exec bash
# OR
source ~/.bashrc
```

#### Method B: System-wide installation (requires sudo)
```bash
sudo ./chart -bash-completion > /etc/bash_completion.d/chart
exec bash
```

#### Method C: Session-only installation
```bash
# Define completion function for current session
eval "$(./chart -bash-completion)"
```

## Manual Installation

If the automatic methods don't work, you can set up completion manually:

### 1. Create the completion function
```bash
# Save this to ~/.bashrc or run directly
_chart_completion() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local completions
    
    # Get completions from the chart command
    completions=$(./chart -complete $COMP_CWORD "${COMP_WORDS[@]:1}" 2>/dev/null)
    
    if [[ -n "$completions" ]]; then
        COMPREPLY=($(compgen -W "$completions" -- "$cur"))
        return 0
    fi
    
    # Fallback to file completion for non-flags
    if [[ $cur != -* ]]; then
        COMPREPLY=($(compgen -f -- "$cur"))
        return 0
    fi
    
    # Complete flag names
    local flags="-x -y -right -title -type -width -height -argv -help -man"
    COMPREPLY=($(compgen -W "$flags" -- "$cur"))
}

# Register the completion
complete -F _chart_completion chart
```

### 2. Test the completion
```bash
# These should work after setup:
chart -y <TAB>          # Shows: time cpu_usage memory_usage disk_io network_rx
chart -y cpu<TAB>       # Completes to: cpu_usage
chart -type <TAB>       # Shows flag options
chart examples/<TAB>    # Shows files in examples/ directory
```

## Verification

To verify completion is working:

```bash
# Check if completion is registered
complete -p chart

# Test field completion
chart -complete 1 -y cpu examples/chart/testdata/sample.tsv
# Should output: cpu_usage

# Test all fields
chart -complete 1 -x "" examples/chart/testdata/sample.tsv  
# Should output: time cpu_usage memory_usage disk_io network_rx
```

## Examples of Working Completion

After setup, you can use tab completion like this:

```bash
# Complete field names after -y flag
$ chart -y <TAB>
time  cpu_usage  memory_usage  disk_io  network_rx

# Partial field name completion
$ chart -y cpu<TAB>
$ chart -y cpu_usage    # <-- completed automatically

# Complete flags
$ chart -<TAB>
-x  -y  -right  -title  -type  -width  -height  -argv  -help

# File completion
$ chart examples/<TAB>
examples/chart/  examples/testdata/

# TSV file completion prioritized
$ chart <TAB>
sample.tsv  other_files...
```

## Troubleshooting

### Completion not working?

1. **Check bash-completion is installed:**
   ```bash
   # Ubuntu/Debian
   sudo apt install bash-completion
   
   # CentOS/RHEL
   sudo yum install bash-completion
   ```

2. **Verify the chart binary path:**
   ```bash
   # Make sure chart is in your PATH or use full path
   which chart
   # Or use relative path: ./chart
   ```

3. **Test completion manually:**
   ```bash
   ./chart -complete 1 -y examples/chart/testdata/sample.tsv
   # Should show field names
   ```

4. **Check function is defined:**
   ```bash
   type _chart_completion
   # Should show the function definition
   ```

5. **Re-source completion:**
   ```bash
   source ~/.bashrc
   # Or restart terminal
   ```

## Advanced Usage

The completion system supports sophisticated features that exceed even the original TSVTools:

### Field-Aware Completion
- **TSV field names** automatically detected from data files
- **Partial matching** for field names  
- **Context-sensitive suggestions** based on previous flags

### Content-Aware Completion
- **Multi-argument switches** like `-match field value` show actual field values from TSV data
- **Real data completion** - shows actual unique values from the specified field
- **Smart caching** - field values are cached for performance

### Universal Switch Negation
- **Any switch** can be prefixed with `+` for negation or `-` for positive
- **Tab completion works** for both `+switch` and `-switch` syntax
- **Consistent semantics** across all switch types

### File Completion with Glob Patterns
- **Suffix filtering** with `suffix=.tsv` in struct tags
- **Character classes** like `suffix=.[tc]sv` for `.tsv` or `.csv` files
- **Brace expansion** like `suffix=.{tsv,csv}` 
- **Wildcard patterns** like `suffix=*.json`
- **Directory navigation** - directories always shown for traversal
- **Smart ordering** - files first, then directories (alphabetically sorted by bash)

### Examples of Advanced Completion

```bash
# Multi-argument switch completion
chart -match <TAB>           # Shows field names: time cpu_usage memory_usage disk_io
chart -match cpu_usage <TAB> # Shows actual values: 25.5 35.0 45.0 30.0 40.0

# Switch negation completion
chart +<TAB>                 # Shows all available switches with + prefix
chart +y <TAB>               # Shows field names for negated Y axis

# File filtering completion
chart -argv <TAB>            # Shows only .tsv/.csv files + directories
chart -config <TAB>          # Shows only .json files + directories

# Directory traversal
chart -argv /tmp/<TAB>       # Shows matching files in /tmp + subdirectories
```

This provides a more sophisticated completion experience than the original TSVTools!

## Struct Tag Configuration for Completion

### Multi-Argument Switches
```go
type Config struct {
    Match []map[string]interface{} `gs:"multi,local,list,args=field:content,help=Filter by field matching content"`
}
```
- Creates switches like `-match field value` that take two arguments
- First argument gets field name completion
- Second argument gets content completion from actual TSV field values

### File Completion with Suffix Filtering
```go
type Config struct {
    // Simple suffix
    TSVFile   string `gs:"file,global,last,suffix=.tsv"`
    
    // Character class (matches .tsv or .csv)
    DataFile  string `gs:"file,global,last,suffix=.[tc]sv"`
    
    // Brace expansion (matches .tsv or .csv)
    InputFile string `gs:"file,global,last,suffix=.{tsv,csv}"`
    
    // Wildcard pattern
    ConfigFile string `gs:"file,global,last,suffix=*.json"`
}
```

### Universal Switch Negation
Any field automatically supports both `+switch` and `-switch` syntax:
```go
type Config struct {
    Y []string `gs:"field,local,list,help=Y-axis fields"`
}

# Both work automatically:
# chart -y cpu_usage    # positive (include)
# chart +y cpu_usage    # negative (exclude)
```

### Enumerated String Completion
String fields can define a fixed set of valid values:
```go
type Config struct {
    Type string `gs:"string,global,last,enum=bar:line:area,default=bar"`
}

# Completion shows only valid enum values:
# chart -type <TAB>     # Shows: bar line area
# chart -type l<TAB>    # Completes to: line
```
- Use `:` to separate enum values (not `,` which conflicts with tag parsing)
- Supports partial matching with case-insensitive prefix matching
- Integrates with validation - invalid enum values are rejected