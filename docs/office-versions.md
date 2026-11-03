# Saves from two tabs

Two office tabs can save the same file. The idle choices (`/api/idle`) and the
layout (`PUT /api/layout`) carry a version so the second save does not wipe
the first.

## The contract

- A read answers an `ETag` header: a short hash of the file bytes as stored,
  `"none"` when the file does not exist. The page reads it from the same
  origin, no CORS headers are added.
- `GET /api/layout` and `POST /api/layout/use` answer the version of the file
  a save would write. For a template or a built-in that is its `-custom`
  copy, so `"none"` until the first save.
- A save may send `If-Match` with the version it loaded. Quotes and a `W/`
  prefix are ignored.
- If the version differs from the file's current one, nothing is written and
  the answer is 409 with a JSON body and the current `ETag`:
  - idle: `{"error":"...","rules":{...},"version":"..."}`
  - layout: `{"error":"...","layout":{...},"active":"<name>","version":"..."}`
- A save without `If-Match` is accepted as before, so other clients keep
  working. A good save answers the new `ETag`.
- The read, compare and write happen under one lock, so two saves on one
  version cannot both win. The edit guards (token, method, edit header,
  origin, content type, body size) run first. A layout save answers its
  other 400 and 409 (format) refusals before the version is compared.

## What the page does

- Idle: on 409 the store takes the server's rules and version, applies only
  the one pending change on top, and saves again (up to 3 tries), then shows
  the merged rules everywhere.
- Editor: on 409 nothing is overwritten. The status reads "This layout
  changed in another tab." with two buttons: "Load theirs" puts the stored
  layout in the editor, "Keep mine" saves again on the version the server
  just answered.
