package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mrkayhyun/gito/internal/i18n"
	"github.com/mrkayhyun/gito/internal/ui"
)

// extraCommands are top-level commands handled in main() rather than
// dispatch(), so they are not in ui.MenuItems but still deserve completion.
var extraCommands = []struct{ name, descKey string }{
	{"completion", "help.completion"},
	{"menu", "help.menu"},
	{"version", "help.version"},
	{"help", "help.help"},
}

var errCompletionUsage = errors.New("usage: gito completion bash|zsh|fish")

// writeCompletion prints the completion script for the shell named in args.
// Command names come from ui.MenuItems, so new commands are picked up
// automatically.
func writeCompletion(w io.Writer, args []string) error {
	if len(args) != 1 {
		return errCompletionUsage
	}
	var script string
	switch args[0] {
	case "bash":
		script = bashCompletion()
	case "zsh":
		script = zshCompletion()
	case "fish":
		script = fishCompletion()
	default:
		return errCompletionUsage
	}
	_, err := io.WriteString(w, script)
	return err
}

type completionCmd struct{ name, desc string }

func completionCommands() []completionCmd {
	var cmds []completionCmd
	for _, item := range ui.MenuItems {
		cmds = append(cmds, completionCmd{item.Key, item.Desc()})
	}
	for _, c := range extraCommands {
		cmds = append(cmds, completionCmd{c.name, i18n.T(c.descKey)})
	}
	return cmds
}

func bashCompletion() string {
	var names []string
	for _, c := range completionCommands() {
		names = append(names, c.name)
	}
	return fmt.Sprintf(`# bash completion for gito
# Load with:  source <(gito completion bash)
_gito() {
    local cur=${COMP_WORDS[COMP_CWORD]}
    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=( $(compgen -W "%s" -- "$cur") )
        return
    fi
    case "${COMP_WORDS[1]}" in
        completion)
            [ "$COMP_CWORD" -eq 2 ] && COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
            ;;
        log|blame)
            COMPREPLY=( $(compgen -f -- "$cur") )
            ;;
    esac
}
complete -o filenames -F _gito gito
`, strings.Join(names, " "))
}

func zshCompletion() string {
	var sb strings.Builder
	for _, c := range completionCommands() {
		// _describe splits on the first unescaped colon.
		desc := strings.ReplaceAll(c.desc, ":", `\:`)
		fmt.Fprintf(&sb, "        %s\n", shellSingleQuote(c.name+":"+desc))
	}
	return fmt.Sprintf(`#compdef gito
# zsh completion for gito
# Load with:  source <(gito completion zsh)
# or save as _gito in a directory on $fpath.
_gito() {
    local -a cmds
    cmds=(
%s    )
    if (( CURRENT == 2 )); then
        _describe 'command' cmds
        return
    fi
    case $words[2] in
        completion) (( CURRENT == 3 )) && _values 'shell' bash zsh fish ;;
        log|blame) _files ;;
    esac
}
if [ "$funcstack[1]" = "_gito" ]; then
    _gito "$@"
else
    compdef _gito gito
fi
`, sb.String())
}

func fishCompletion() string {
	var sb strings.Builder
	sb.WriteString("# fish completion for gito\n")
	sb.WriteString("# Load with:  gito completion fish | source\n")
	sb.WriteString("complete -c gito -f\n")
	for _, c := range completionCommands() {
		fmt.Fprintf(&sb, "complete -c gito -n __fish_use_subcommand -a %s -d %s\n",
			shellSingleQuote(c.name), shellSingleQuote(c.desc))
	}
	sb.WriteString("complete -c gito -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'\n")
	sb.WriteString("complete -c gito -n '__fish_seen_subcommand_from log blame' -F\n")
	return sb.String()
}

// shellSingleQuote quotes s for POSIX shells, zsh and fish alike: wrap in
// single quotes and close/escape/reopen around embedded single quotes. That
// close-escape-reopen form is valid in all three shells.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
