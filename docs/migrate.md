# Copying profile history into the shared folder

A profile folder (`~/.config/clalt/profiles/<name>`) can hold its own
conversation history in `projects/`. The shared history is `~/.claude/projects`,
or `$CLAUDE_CONFIG_DIR/projects` when that variable is set to a folder outside
clalt's own. `clalt migrate` copies each profile's history into it, so those
conversations can be resumed from the shared folder:

```
clalt migrate --dry-run    # show what would happen, write nothing
clalt migrate              # copy
```

It prints one line per profile and a total (`--json` gives the same as a
report). Run it from a normal shell, not inside a `clalt run` session: there
`CLAUDE_CONFIG_DIR` points at an account folder, and clalt refuses. It is safe to
run again, and a second run copies nothing:

- It only copies. The files in the profile folders stay where they are, and
  nothing is changed or deleted there.
- Nothing in the shared folder is ever overwritten. A file that is already
  there with the same content counts as "already there". One that is there with
  different content is a conflict: it is listed by path with the reason, and the
  profile's version stays only in the profile folder.
- A `projects` folder that is a link is never followed. A link to the shared
  folder is reported as "already shared", a link anywhere else is reported and
  not copied, and so is a profile folder that is itself a link. Links inside a
  history folder are skipped and listed.
- A dry run reads and compares but writes nothing, and its report is what a real
  run would do, including when two profiles hold the same path. It does not
  check free space or write permission.
- Copied files keep their modification time (Claude Code sorts its resume list
  by it) and are readable only by you.
- A file only appears under its real name once it is complete. If a run is
  killed part way, the file being copied can leave a hidden `.clalt-migrate-*`
  file in its folder in the shared folder. It is safe to delete, and a new run
  finishes the copy. On a filesystem without hard links, such as FAT, the file is
  written in place instead, and then a kill part way can leave a partial file
  under its real name. The report counts the files written that way.

If a file cannot be copied the rest still are, the failures are listed, and the
exit code is 1. File names in the text output are escaped, so a name cannot
forge a line or move the cursor.
