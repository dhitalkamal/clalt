# Upgrading from v0.1.0

v0.1.0 kept every account's token in its own encrypted vault. Nothing reads
that vault any more, so:

1. Sign each account in again: `clalt login <name>`. Accounts keep their
   names, so existing names work as they are.
2. Run `clalt migrate` to copy the old per-account history into the shared
   history (see [migrate.md](migrate.md)).
3. Delete what you no longer need. clalt never deletes anything itself, and
   `clalt doctor` lists what is still there:
   - the old vault: `~/.config/clalt/vault/` and `~/.config/clalt/vault.key`
     (under `$XDG_CONFIG_HOME/clalt/` if you set it);
   - the old profile folders in `~/.config/clalt/profiles/`. v0.1.0 staged a
     login there as plain text, in a `.credentials.json` in some of the
     folders. `clalt doctor` names each one it finds (it only checks that the
     file exists and never opens it), and those files can go whether or not you
     migrate. The rest of a folder is history: delete the folder once `migrate`
     says everything is copied. Doctor warns for as long as a folder is there;
   - the old keepalive job, if it was installed: `launchctl unload` and delete
     `~/Library/LaunchAgents/com.clalt.keepalive.plist` on macOS,
     `systemctl --user disable --now com.clalt.keepalive.timer` and delete the
     `com.clalt.keepalive.service` and `.timer` units on Linux, or
     `schtasks /Delete /TN com.clalt.keepalive /F` on Windows, where the job was
     a scheduled task that no file shows. The job calls `clalt keepalive`, which
     is gone, so until it is removed it only fails;
   - the marker files `keepalive.auto` and `keepalive.disabled` in
     `~/.config/clalt/`.

   Right after the upgrade `clalt doctor` exits 0, with a warning for each
   account that is not signed in yet and for each of these leftovers.

An upgraded account keeps the email that v0.1.0 recorded for it. `clalt login
<name>` passes that email to claude and refuses a sign-in as another one. To use
another email under the same name, `clalt remove <name>` first, which forgets
the email, and then sign in again.

`clalt list` no longer marks the active account with a `*`. There is no active
account any more: every account can run at once, and `clalt whoami` says which
account the shell is in.

These environment variables of v0.1.0 are not read any more: `CLALT_NO_BROWSER`,
`CLALT_OAUTH_CLIENT_ID`, `CLALT_OAUTH_SCOPES`, `CLALT_AUTHORIZE_URL`,
`CLALT_TOKEN_URL`, `CLALT_REDIRECT_URI`, `CLALT_API_BASE` and
`CLALT_NO_AUTO_KEEPALIVE`. `CLALT_CLAUDE_BIN` and `CLALT_ASCII` still work.

v0.1.0 let two accounts have names that differ only in case, like `Robin` and
`robin`. On a file system that ignores case (macOS and Windows by default) they
would be one folder, and so one login. `clalt doctor` reports such accounts, and
`login` and `run` refuse them until you `clalt remove` one of the two and sign
the other in again.

`state.json` is still readable by the v0.1.0 binary, in case you have to go
back. Rewriting it there would drop the new fields, which is accepted for an
emergency rollback.

Gone, with a one-line message that says what replaces it: `clalt add`,
`refresh`, `keepalive`, `export` and `import` (accounts hold no secrets now),
`--live` on `usage` and `dashboard`, `doctor --fix`, `login --magic-link`, and
`run --refresh`, `--isolated`, `--no-supervise` and `--link`.
