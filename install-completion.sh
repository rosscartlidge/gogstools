#!/bin/bash

# GoGSTools Chart Completion Installation Script

set -e

CHART_BINARY="./chart"
COMPLETION_FILE="$HOME/.local/share/bash-completion/completions/chart"

echo "Installing bash completion for GoGSTools chart command..."

# Check if chart binary exists
if [[ ! -f "$CHART_BINARY" ]]; then
    echo "Building chart binary..."
    if command -v go >/dev/null 2>&1; then
        export GOROOT=/usr/lib/go
        go build -o chart ./examples/chart
    else
        echo "Error: Go is not installed or chart binary not found"
        exit 1
    fi
fi

# Create completion directory
mkdir -p "$(dirname "$COMPLETION_FILE")"

# Generate and install completion
echo "Generating completion script..."
$CHART_BINARY -bash-completion > "$COMPLETION_FILE"

echo "Completion installed to: $COMPLETION_FILE"
echo ""
echo "To activate completion, run one of the following:"
echo ""
echo "  # Option 1: Restart your shell"
echo "  exec bash"
echo ""
echo "  # Option 2: Source the completion file directly"
echo "  source $COMPLETION_FILE"
echo ""
echo "  # Option 3: Enable for current session only"
echo "  eval \"\$(./chart -bash-completion)\""
echo ""
echo "After activation, you can use tab completion:"
echo "  chart -y <TAB>                    # Shows TSV field names"
echo "  chart -y cpu<TAB>                # Completes to cpu_usage"
echo "  chart -type <TAB>                # Shows flag options"
echo "  chart examples/<TAB>             # Shows files"
echo ""