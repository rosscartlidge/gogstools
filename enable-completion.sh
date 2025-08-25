#!/bin/bash

# One-liner script to enable chart completion for current session

set -e

# Build if needed
if [[ ! -f "./chart" ]]; then
    echo "Building chart binary..."
    export GOROOT=/usr/lib/go
    go build -o chart ./examples/chart
fi

CHART_PATH="$(realpath ./chart)"

echo "Enabling tab completion for chart command..."
echo "Chart binary: $CHART_PATH"

# Create and load completion function
eval "$(cat << EOF
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
)"

echo "✅ Tab completion enabled for current session!"
echo ""
echo "Try these examples:"
echo "  chart -y <TAB>"
echo "  chart -argv /tmp/xxx.tsv -x <TAB>"
echo "  chart -y mem<TAB>"
echo ""
echo "To make this permanent, add the completion function to your ~/.bashrc"