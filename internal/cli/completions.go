package cli

import (
	"fmt"
	"io"
)

// printCompletions writes the completion script of the supported shells.
func printCompletions(shell string, out io.Writer) int {
	switch shell {
	case "bash":
		fmt.Fprint(out, bashCompletion)
	case "zsh":
		fmt.Fprint(out, zshCompletion)
	case "fish":
		fmt.Fprint(out, fishCompletion)
	default:
		fmt.Fprintf(out, "unbekannte Shell: %s (erlaubt: bash, zsh, fish)\n", shell)
		return 1
	}
	return 0
}

// The option names for the completion scripts.
const optionNames = `--list --format --episode --range --year --since --title --all --quality --layout
--parallel --delay --thumb --meta --totp --library --json --check --download-new --add
--automatic --links-file --force-redownload --completions --man --help --version`

const bashCompletion = `# orangeflix-dl bash completion
_orangeflix_dl_completions() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local prev="${COMP_WORDS[COMP_CWORD-1]}"
  local opts="` + "`" + `echo "` + optionNames + `"` + "`" + `"
  case "${prev}" in
    --format|--quality|--layout|--add|--links-file|--totp|--completions)
      COMPREPLY=()
      return 0
      ;;
  esac
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
  return 0
}
complete -o default -F _orangeflix_dl_completions orangeflix-dl
`

const zshCompletion = `#compdef orangeflix-dl
# orangeflix-dl zsh completion
_orangeflix_dl() {
  local -a opts
  opts=(` + optionNames + `)
  _arguments -S : \
    '--format[Format wählen (Slug, Name oder ID)]:format:' \
    '--episode[einzelne Folge]:nummer:' \
    '--range[Folgenbereich a-b]:bereich:' \
    '--year[alle Folgen eines Jahres]:jahr:' \
    '--since[alle Folgen ab dem Tag]:datum:' \
    '--title[Titel enthält den Text]:text:' \
    '--quality[Qualität best, hd, sd, mobile, audio]:qualität:(best hd sd mobile audio)' \
    '--layout[compat oder flat]:layout:(compat flat)' \
    '--parallel[Downloads gleichzeitig 1-8]:zahl:' \
    '--delay[Pause zwischen Downloads]:sekunden:' \
    '--totp[zweiter Faktor]:code:' \
    '--add[Format merken]:format:' \
    '--links-file[Formate aus einer Datei]:datei:_files' \
    '--completions[Kompletion einer Shell]:shell:(bash zsh fish)' \
    "--library --check --download-new --list --all --thumb --meta --json --automatic --force-redownload --man --help --version"
  _describe 'options' opts
}
_orangeflix_dl "$@"
`

const fishCompletion = `# orangeflix-dl fish completion
complete -c orangeflix-dl -f
complete -c orangeflix-dl -l format -l add -l links-file -l totp -l completions -r -d 'Wert'
complete -c orangeflix-dl -l episode -l range -l year -l since -l title -l quality -l layout -l parallel -l delay -r -d 'Wert'
for opt in ` + optionNames + `
  if not test "$opt" = --format; and not test "$opt" = --add; and not test "$opt" = --links-file; and not test "$opt" = --totp; and not test "$opt" = --completions; and not test "$opt" = --episode; and not test "$opt" = --range; and not test "$opt" = --year; and not test "$opt" = --since; and not test "$opt" = --title; and not test "$opt" = --quality; and not test "$opt" = --layout; and not test "$opt" = --parallel; and not test "$opt" = --delay
    complete -c orangeflix-dl -a "$opt"
  end
end
`
