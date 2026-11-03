# Opening the office from another device

The office only listens on `127.0.0.1`. To reach it from a phone or another
computer, put it behind a Cloudflare Tunnel with Cloudflare Access in front.
With an `access.json` in place, the office trusts Access as the login on that
one public hostname, so the link needs no `?t=` token there.

## What you need

- A Cloudflare Tunnel that sends your hostname (say `clalt.example.com`) to
  `http://127.0.0.1:47321`. Do not set `httpHostHeader` on it: the office has
  to see the public hostname to know the request came through Access.
- A Cloudflare Access self-hosted application for that hostname, with a policy
  that allows your email (the One-time PIN login method works).

## access.json

Put this in `~/.config/clalt/office/access.json`, then restart the office
(`clalt office off`, then `clalt office on`):

```json
{
  "host": "clalt.example.com",
  "team": "your-team",
  "aud": "the application audience tag",
  "email": "you@example.com"
}
```

- `host` is the public hostname, no scheme and no port.
- `team` is the part before `.cloudflareaccess.com` in your Zero Trust team
  domain.
- `aud` is the Application Audience (AUD) Tag on the overview of the Access
  application.
- `email` is the one person let in. If the Access policy later allows more
  people, they still cannot open the office.

No file means the feature is off. A file that is broken or incomplete also
leaves it off and writes one line to `~/.config/clalt/logs/office.log`.

## What it checks

Every request for the public hostname must carry the `Cf-Access-Jwt-Assertion`
header that Access adds. The office accepts it only when:

- it is signed with RS256 by a current key from
  `https://<team>.cloudflareaccess.com/cdn-cgi/access/certs`,
- its audience is `aud`, its issuer is the team domain, and it is in date,
- its email is `email` (case does not matter).

Anything else is refused with 403, the same as an unknown hostname. The keys
are cached and fetched again at most once a minute when a new key id shows up.
If they cannot be fetched, the public hostname is refused until they can: it
never falls back to letting a request in.

The assertion counts on the public hostname only. On `127.0.0.1` the token is
still needed, as before. Saving a layout or using a session action from the
public page works when the request carries both the assertion and the
`https://` origin of the public hostname. Refusals are logged with the reason,
never with the assertion itself.
