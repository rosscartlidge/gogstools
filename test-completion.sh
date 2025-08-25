#!/bin/bash

# Test script to verify bash completion works

set -e

echo "=== Testing GoGSTools Chart Completion ==="
echo

# Build if needed
if [[ ! -f "./chart" ]]; then
    echo "Building chart binary..."
    export GOROOT=/usr/lib/go
    go build -o chart ./examples/chart
fi

CHART_PATH="$(pwd)/chart"
echo "Chart binary: $CHART_PATH"

# Test completion engine directly
echo
echo "1. Testing completion engine directly:"
echo "   $CHART_PATH -complete 3 -argv /tmp/xxx.tsv -x"
$CHART_PATH -complete 3 -argv /tmp/xxx.tsv -x
echo

# Create completion script with full path
echo "2. Creating completion script with full path..."
cat > /tmp/chart_completion_test.sh << EOF
# Bash completion for chart
_chart_completion() {
    local cur prev
    
    # Basic completion setup
    cur="\${COMP_WORDS[COMP_CWORD]}"
    prev="\${COMP_WORDS[COMP_CWORD-1]}"
    
    # Call the command with -complete to get suggestions  
    local completions
    completions=(\$($CHART_PATH -complete \$COMP_CWORD "\${COMP_WORDS[@]:1}" 2>/dev/null))
    
    # Generate completion replies from command output
    if [[ \${#completions[@]} -gt 0 ]]; then
        COMPREPLY=(\$(compgen -W "\${completions[*]}" -- "\$cur"))
        return 0
    fi
    
    # Handle file completion for arguments that aren't flags
    if [[ \$cur != -* ]]; then
        # Complete TSV files specifically, then other files
        local tsv_files=(\$(compgen -f -X '!*.tsv' -- "\$cur" 2>/dev/null))
        local all_files=(\$(compgen -f -- "\$cur" 2>/dev/null))
        COMPREPLY=("\${tsv_files[@]}" "\${all_files[@]}")
        return 0
    fi
    
    # Complete flag names
    local flags="-x -y -right -title -type -width -height -argv -help -man -complete -bash-completion"
    COMPREPLY=(\$(compgen -W "\$flags" -- "\$cur"))
}

# Register the completion function
complete -F _chart_completion chart
EOF

echo "3. Loading completion script..."
source /tmp/chart_completion_test.sh

echo "4. Verifying completion is registered..."
if complete -p chart >/dev/null 2>&1; then
    echo "✅ Completion registered successfully!"
    complete -p chart
else
    echo "❌ Completion registration failed"
    exit 1
fi

echo
echo "5. Manual completion test:"
echo "   Simulating: chart -argv /tmp/xxx.tsv -x <TAB>"

# Simulate what bash completion would do
COMP_WORDS=("chart" "-argv" "/tmp/xxx.tsv" "-x")
COMP_CWORD=3
COMPREPLY=()

_chart_completion

echo "   Completion results: \${COMPREPLY[@]}"
for completion in "\${COMPREPLY[@]}"; do
    echo "   - \$completion"
done

echo
echo "=== Completion Test Complete ==="
echo
echo "To manually test tab completion:"
echo "1. Source the completion script:"
echo "   source /tmp/chart_completion_test.sh"
echo "2. Try tab completion:"
echo "   chart -argv /tmp/xxx.tsv -x <TAB>"
echo "   chart -y <TAB>"
echo "   chart -argv /tmp/xxx.tsv -y mem<TAB>"