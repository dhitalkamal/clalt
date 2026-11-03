# Security

## Reporting a problem

Please do not open a public issue for a security problem. Use GitHub's private
reporting instead: on the repository page, open Security, then "Report a
vulnerability". Only the maintainer sees the report.

Include what you found, how to reproduce it, the clalt version (`clalt
version`) and your OS. You should get an answer within a week.

## What is in scope

- the office server (`clalt office`): the run token, the loopback-only
  listener, and the opt-in remote login through Cloudflare Access
  (docs/office-remote.md)
- anything that makes clalt read or print a credential, a token or the
  keychain, which it is designed never to do
- the MCP server (`clalt mcp`) and the hooks `clalt feed install` adds

Problems in claude itself go to Anthropic, not here.

## Supported versions

Only the latest release gets fixes.
