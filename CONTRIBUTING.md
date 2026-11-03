# Contributing

Bug reports, ideas and pull requests are welcome.

## Reporting a bug

Open an issue with `clalt version`, `claude --version`, your OS, what you ran
and what happened. For the office, say which template and browser, and paste
any console error. Security problems go through SECURITY.md instead.

## Pull requests

- branch off `develop` and open the pull request against `develop`
- branch names: `feat/<slug>`, `fix/<slug>` or `chore/<slug>`
- commit messages: imperative, lowercase, under 72 characters
- add a test that fails without your change; behaviour changes need one
- run `make check` before pushing (gofmt, vet, Go tests, the office JS tests
  and the build)
- keep one concern per pull request

## Tests never touch your real setup

Tests run against a temp HOME, temp state dirs and a fake claude script
(`CLALT_CLAUDE_BIN`). Never make a test read `~/.claude`, `~/.config/clalt` or
the keychain, or run the real claude.

## Style

- ASCII only in files, no decorative comments or banners
- comments say why, in plain lowercase
- files stay under 500 lines; split before they get there
