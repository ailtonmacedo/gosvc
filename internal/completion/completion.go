package completion

import (
	"fmt"
	"strings"
)

var commands = []string{
	"version", "validate-config", "validate", "doctor", "check", "verify",
	"new", "add", "upgrade", "plugins", "completion", "release", "acceptance", "certify", "help",
}

// Generate returns a shell completion script for gosvc.
func Generate(shell string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "bash":
		return bash(), nil
	case "zsh":
		return zsh(), nil
	case "fish":
		return fish(), nil
	case "powershell", "pwsh":
		return powershell(), nil
	default:
		return "", fmt.Errorf("unsupported shell %q; supported shells: bash, zsh, fish, powershell", shell)
	}
}

func bash() string {
	return `# bash completion for gosvc
_gosvc_completion() {
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "version validate-config validate doctor check verify new add upgrade plugins completion release acceptance certify help" -- "$cur") )
    return 0
  fi

  case "${COMP_WORDS[1]}" in
    check)
      COMPREPLY=( $(compgen -W "architecture --project" -- "$cur") ) ;;
    add)
      COMPREPLY=( $(compgen -W "resource --fields --crud --project --dry-run --force" -- "$cur") ) ;;
    plugins)
      COMPREPLY=( $(compgen -W "list validate checksum run --project --dry-run --force --timeout" -- "$cur") ) ;;
    release)
      COMPREPLY=( $(compgen -W "prepare github-plan check snapshot verify --project --repository --version --output --dist --parallel --allow-placeholder --dry-run --skip-exec --json" -- "$cur") ) ;;
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish powershell" -- "$cur") ) ;;
    acceptance)
      COMPREPLY=( $(compgen -W "--workdir --keep --json" -- "$cur") ) ;;
    certify)
      COMPREPLY=( $(compgen -W "--mode static real --workdir --keep --json --require-real --timeout" -- "$cur") ) ;;
    new)
      COMPREPLY=( $(compgen -W "--module --preset --output --dry-run --force minimal-api postgres-api production-api event-driven-api" -- "$cur") ) ;;
    validate|doctor|verify)
      COMPREPLY=( $(compgen -W "--project --static" -- "$cur") ) ;;
    upgrade)
      COMPREPLY=( $(compgen -W "backups rollback notes --project --to --from --backup --dry-run --force --no-backup" -- "$cur") ) ;;
  esac
}
complete -F _gosvc_completion gosvc
`
}

func zsh() string {
	return `#compdef gosvc

_gosvc() {
  local -a commands
  commands=(
    'version:show build version information'
    'validate-config:validate a project.yaml'
    'validate:validate a generated project'
    'doctor:check required tools'
    'check:run architecture checks'
    'verify:run project verification'
    'new:create or regenerate a project'
    'add:add a CRUD resource'
    'upgrade:upgrade, back up, or roll back a generated project'
    'plugins:manage external plugins'
    'completion:generate shell completion'
    'release:prepare, check, build, or verify release assets'
    'acceptance:generate and validate the preset matrix'
    'certify:run static or real integration certification'
    'help:show help'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi

  case $words[2] in
    completion) _values 'shell' bash zsh fish powershell ;;
    check) _values 'check' architecture ;;
    add) _values 'resource command' resource ;;
    plugins) _values 'plugin command' list validate checksum run ;;
    release) _values 'release command' prepare github-plan check snapshot verify ;;
    upgrade) _values 'upgrade command' backups rollback notes ;;
  esac
}

_gosvc "$@"
`
}

func fish() string {
	var b strings.Builder
	b.WriteString("# fish completion for gosvc\n")
	for _, command := range commands {
		fmt.Fprintf(&b, "complete -c gosvc -n '__fish_use_subcommand' -a %s\n", command)
	}
	b.WriteString("complete -c gosvc -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish powershell'\n")
	b.WriteString("complete -c gosvc -n '__fish_seen_subcommand_from check' -a architecture\n")
	b.WriteString("complete -c gosvc -n '__fish_seen_subcommand_from add' -a resource\n")
	b.WriteString("complete -c gosvc -n '__fish_seen_subcommand_from plugins' -a 'list validate checksum run'\n")
	b.WriteString("complete -c gosvc -n '__fish_seen_subcommand_from release' -a 'prepare github-plan check snapshot verify'\n")
	b.WriteString("complete -c gosvc -n '__fish_seen_subcommand_from upgrade' -a 'backups rollback notes'\n")
	return b.String()
}

func powershell() string {
	return `# PowerShell completion for gosvc
Register-ArgumentCompleter -Native -CommandName gosvc -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  $tokens = $commandAst.CommandElements | ForEach-Object { $_.Extent.Text }
  $values = @('version','validate-config','validate','doctor','check','verify','new','add','upgrade','plugins','completion','release','acceptance','certify','help')
  if ($tokens.Count -ge 2) {
    switch ($tokens[1]) {
      'completion' { $values = @('bash','zsh','fish','powershell') }
      'check'      { $values = @('architecture','--project') }
      'add'        { $values = @('resource','--fields','--crud','--project','--dry-run','--force') }
      'plugins'    { $values = @('list','validate','checksum','run','--project','--dry-run','--force','--timeout') }
      'release'    { $values = @('prepare','github-plan','check','snapshot','verify','--project','--repository','--version','--output','--dist','--parallel','--allow-placeholder','--dry-run','--skip-exec') }
      'upgrade'    { $values = @('backups','rollback','notes','--project','--to','--from','--backup','--dry-run','--force','--no-backup') }
      'acceptance' { $values = @('--workdir','--keep','--json') }
      'certify'    { $values = @('--mode','static','real','--workdir','--keep','--json','--require-real','--timeout') }
    }
  }
  $values | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
  }
}
`
}
