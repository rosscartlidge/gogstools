# How to Install Chart Tab Completion

## Step-by-Step Installation

### 1. Build the Chart Binary
```bash
cd /path/to/gogstools
export GOROOT=/usr/lib/go  # Adjust if needed
go build -o chart ./examples/chart
```

### 2. Install Completion (Choose One Method)

#### Method A: Quick Test (Current Session Only)
```bash
# Create completion script with full path
CHART_PATH="$(pwd)/chart"
cat > ~/.chart_completion.sh << EOF
_chart_completion() {
    local cur="\${COMP_WORDS[COMP_CWORD]}"
    local completions
    completions=(\$($CHART_PATH -complete \$COMP_CWORD "\${COMP_WORDS[@]:1}" 2>/dev/null))
    
    if [[ \${#completions[@]} -gt 0 ]]; then
        COMPREPLY=(\$(compgen -W "\${completions[*]}" -- "\$cur"))
        return 0
    fi
    
    if [[ \$cur != -* ]]; then
        COMPREPLY=(\$(compgen -f -X '!*.tsv' -- "\$cur" 2>/dev/null))
        return 0
    fi
    
    local flags="-x -y -right -title -type -width -height -argv -help"
    COMPREPLY=(\$(compgen -W "\$flags" -- "\$cur"))
}
complete -F _chart_completion chart
EOF

# Load it
source ~/.chart_completion.sh

# Test it
chart -argv /tmp/xxx.tsv -x <TAB>
```

#### Method B: Permanent Installation
```bash
# Copy binary to PATH
sudo cp chart /usr/local/bin/chart

# Install system-wide completion
sudo tee /etc/bash_completion.d/chart << 'EOF'
_chart_completion() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local completions
    completions=($(chart -complete $COMP_CWORD "${COMP_WORDS[@]:1}" 2>/dev/null))
    
    if [[ ${#completions[@]} -gt 0 ]]; then
        COMPREPLY=($(compgen -W "${completions[*]}" -- "$cur"))
        return 0
    fi
    
    if [[ $cur != -* ]]; then
        COMPREPLY=($(compgen -f -X '!*.tsv' -- "$cur" 2>/dev/null))
        return 0
    fi
    
    local flags="-x -y -right -title -type -width -height -argv -help"
    COMPREPLY=($(compgen -W "$flags" -- "$cur"))
}
complete -F _chart_completion chart
EOF

# Restart shell or source completion
exec bash
```

### 3. Verify Installation
```bash
# Check completion is registered
complete -p chart

# Test various completion scenarios
chart -y <TAB>                              # Should show field names
chart -argv /tmp/xxx.tsv -x <TAB>     # Should show field names from xxx.tsv  
chart -y mem<TAB>                           # Should complete to memory_usage
chart -type <TAB>                           # Should show flag options
```

## Troubleshooting

### Tab Completion Not Working?

1. **Verify completion is loaded:**
   ```bash
   complete -p chart
   # Should output: complete -F _chart_completion chart
   ```

2. **Test completion engine directly:**
   ```bash
   ./chart -complete 3 -argv /tmp/xxx.tsv -x
   # Should output field names
   ```

3. **Check bash-completion is installed:**
   ```bash
   # Ubuntu/Debian
   sudo apt install bash-completion
   
   # CentOS/RHEL  
   sudo yum install bash-completion
   ```

4. **Manual debug:**
   ```bash
   # Set up debug environment
   COMP_WORDS=("chart" "-argv" "/tmp/xxx.tsv" "-x")
   COMP_CWORD=3
   COMPREPLY=()
   
   # Call completion function
   _chart_completion
   
   # Check results
   echo "Completions: ${COMPREPLY[@]}"
   ```

5. **Create minimal test:**
   ```bash
   # Simple working completion
   _test_completion() {
       COMPREPLY=($(./chart -complete $COMP_CWORD "${COMP_WORDS[@]:1}" 2>/dev/null))
   }
   complete -F _test_completion chart
   
   # Now test: chart -argv /tmp/xxx.tsv -x <TAB>
   ```

## Expected Behavior

After successful installation:

- `chart -x <TAB>` → Shows available field names from TSV files in arguments
- `chart -y cpu<TAB>` → Completes to `cpu_usage`
- `chart -argv data.tsv -y mem<TAB>` → Completes to `memory_usage`
- `chart -type <TAB>` → Shows available flags
- `chart /path/to/<TAB>` → Shows files (prioritizing .tsv files)

The completion system understands the structure of your commands and provides context-aware suggestions based on TSV file contents!