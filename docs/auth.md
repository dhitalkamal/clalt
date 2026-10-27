# Auth environment variables

Before every claude call clalt removes the variables that would override the
folder's own login: seventeen of them. Twelve outrank a subscription login in
claude's precedence, so any of them set in the shell would win over the login of
the account folder:

```
CLAUDE_CODE_OAUTH_TOKEN  CLAUDE_CODE_OAUTH_REFRESH_TOKEN  CLAUDE_CODE_OAUTH_SCOPES
ANTHROPIC_API_KEY  ANTHROPIC_AUTH_TOKEN  ANTHROPIC_PROFILE
ANTHROPIC_FEDERATION_RULE_ID  ANTHROPIC_ORGANIZATION_ID
CLAUDE_SECURESTORAGE_CONFIG_DIR
CLAUDE_CODE_USE_BEDROCK  CLAUDE_CODE_USE_VERTEX  CLAUDE_CODE_USE_FOUNDRY
```

Five more are removed too. They are not in that precedence list, but the first
three switch the provider or the credentials claude signs in with, and the last
two could send the account's token to another host:

```
CLAUDE_CODE_USE_MANTLE  CLAUDE_CODE_USE_ANTHROPIC_AWS  ANTHROPIC_AWS_API_KEY
ANTHROPIC_BASE_URL  ANTHROPIC_CUSTOM_HEADERS
```

`login` and `run` say which of them were set and are not passed on (names only,
never values), on stderr. `doctor` lists the ones set in the shell.

`CLAUDE_CONFIG_DIR` is set to the account folder for every call that is about an
account: login, run, status and logout. The version check, `claude --version`,
is about no folder. It has the variables above removed as well, but
`CLAUDE_CONFIG_DIR` is left as the shell has it.

An `apiKeyHelper`, or an `env` block that sets one of these variables, in the
shared `settings.json` or `settings.local.json` would override every account's
login: the run-time sign-in check catches it, and `clalt doctor` warns about
both.

See also: [doctor.md](doctor.md) for the check that names these variables, and
[commands.md](commands.md) for how `run` and `login` use them.
