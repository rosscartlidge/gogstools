package completion

import (
	"fmt"
)

// GenerateBashCompletion generates a bash completion script for a command
func GenerateBashCompletion(commandName string) string {
	return fmt.Sprintf(`# Bash completion for %s
_%s_completion() {
    local cur prev words cword
    _init_completion || return

    # Get current word being completed
    cur="${COMP_WORDS[COMP_CWORD]}"
    
    # Call the command with -complete to get suggestions
    local completions
    completions=$(%s -complete $((COMP_CWORD-1)) "${COMP_WORDS[@]:1}")
    
    # Generate completion replies
    COMPREPLY=($(compgen -W "$completions" -- "$cur"))
    
    # Handle file completion for non-flag arguments
    if [[ $cur != -* ]] && [[ $prev != -* ]] || [[ $prev == -argv ]]; then
        _filedir
    fi
}

# Register the completion function
complete -F _%s_completion %s
`, commandName, commandName, commandName, commandName, commandName)
}

// InstallBashCompletion provides instructions for installing bash completion
func InstallBashCompletion(commandName string) string {
	return fmt.Sprintf(`# To install bash completion for %s:

## Method 1: User-specific installation
mkdir -p ~/.local/share/bash-completion/completions
%s -bash-completion > ~/.local/share/bash-completion/completions/%s
# Then restart your shell or run: source ~/.local/share/bash-completion/completions/%s

## Method 2: System-wide installation (requires sudo)
sudo %s -bash-completion > /etc/bash_completion.d/%s
# Then restart your shell

## Method 3: Temporary installation (current session only)
eval "$(%s -bash-completion)"

## Usage after installation:
# %s -x <TAB>          # Shows all available fields from TSV files
# %s -y cpu<TAB>       # Completes to cpu_usage
# %s -type <TAB>       # Shows available chart types
# %s <TAB>             # Shows files in current directory
`, commandName, commandName, commandName, commandName, commandName, commandName, commandName, commandName, commandName, commandName, commandName)
}