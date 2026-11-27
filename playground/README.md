# CLI playground

A terminal in the browser for trying the notte CLI and iterating on its output.

```sh
make playground          # live, rebuilds on save: http://127.0.0.1:4173
make playground-export   # static site in dist/playground
```

## How it works

The shell is emulated in the browser (`web/index.html`): `pwd cd ls cat echo
mkdir touch rm mv cp tree grep head tail wc sort uniq jq env export history
clear open`, plus `|`, `>`, `>>`, `2>`, `2>&1`, `&&`, `||`, `;`, quotes,
`$VARS` and `~`, over a pretend filesystem kept in local storage. `reset`
wipes it.

`notte` depends on where the page is served from:

- **`make playground` (live):** `notte` runs the real CLI built from this
  checkout against the real Notte API (`live.go`). The server executes only
  that binary, with the arguments as an argv array, never a shell, and only
  for requests from the playground page itself. Output streams as it
  arrives, typed lines go to the command's prompts, Ctrl+C stops it, Ctrl+D
  closes its input. The files of the current directory are copied into the
  command's working directory, and files it creates come back into the
  pretend filesystem (`open screenshot.png` views one). `NOTTE_*` variables
  exported in the page reach the CLI.
  The CLI has its own home (`<user cache dir>/notte-cli-playground`), so it
  is signed in separately from an installed CLI: run `notte auth login`. Its
  key is kept in the CLI's file credential store there, never the OS
  keychain. Sessions you start are real and billed.
- **`make playground-export` (static):** `notte` is the real CLI compiled to
  WebAssembly (`notte.wasm`, built from this checkout with
  `GOOS=js GOARCH=wasm`) and run in the visitor's browser. No server runs
  anything, so one visitor has nothing to reach of another's. See below.

## The static export

The export directory holds `index.html`, `notte.wasm`, Go's loader
(`wasm_exec.js`) and the help pages the sidebar is built from
(`preview.json`). The page calls the API through its own host, so it needs
the pass-through in `vercel.json`.

It is hosted on Vercel from this repository: a project with `playground` as
its root directory builds every push to `main` with the settings in
`vercel.json` (`vercel-install.sh` installs Go, which the build image lacks).

- **Same code, no port to maintain.** Every command, flag and output format
  comes from the same source as the released binaries. The few pieces a
  browser cannot run are behind `js` build tags: reading a local browser
  profile (`internal/browser`), the file lock of the key store, and the local
  port of `notte auth login` (`internal/auth`). CI builds the browser target,
  so a new dependency that does not compile there fails the pull request.
- **Files.** The CLI reads and writes the page's pretend filesystem
  (`cliSystem` in `web/index.html`), so a saved key, a pinned session or a
  downloaded file is a file `ls` shows.
- **Sign-in.** `notte auth login` prints a console link with a code and
  waits until it is approved, as in a terminal; `notte auth login --with-token`
  reads a pasted key. The key is saved by the CLI's file credential store, in
  the visitor's browser storage for this site, and nowhere else. The page's
  security headers let it connect only to its own host and to S3 (file and
  replay downloads).
- **API.** The API does not answer cross-origin browser requests, so the page
  calls `/notte-api/...` on its own host, which `vercel.json` forwards to
  `https://api.notte.cc` unchanged. The key travels in that request as it
  would from a terminal; the host keeps nothing. Calls carry the request
  origin `cli-playground`, so the API tells them apart from the installed CLI.
- **What cannot work in a browser.** Commands that need this machine: `tunnel`,
  `upgrade`, `skill add`, importing a local browser profile, and uploads or
  downloads to hosts that refuse cross-origin requests. They fail with the
  CLI's own error.

## Iterating on output

Keep `make playground` running and pick a mode in the window's title bar:

- **Fixtures:** `notte` runs the current build against the fake API answers
  in `scenarios.json`, picking the example that matches the command and its
  flags (an empty list, an error, a missing session...). Free, offline and
  stable, so it is the mode for working on formats. After every rebuild, and
  every save of `scenarios.json`, the last command re-runs by itself and the
  earlier output is dimmed above it.
- **Live API:** the same build against the real API, with the playground's
  own login.

Edit output code (generic rendering in `internal/output/text.go`,
command-specific rendering in `internal/cmd/`, `output_helpers.go` in
particular) and save. Only the compile and the help pages run on a save, so
the new output shows within seconds. Editing the page or `scenarios.json`
never rebuilds: the page is read on every load and fixtures on every run.
Build errors are shown above the prompt; the last good build keeps running.

## Adding a recording

Add an entry to `scenarios.json`:

- `id`: unique. End it with `-no-session` for the error a session command
  prints without a session.
- `args`: CLI arguments without `notte`. Pass `--session-id demo-session-001`
  for session commands.
- `responses`: exact API paths with synthetic JSON bodies. The runner fails if
  the command calls a path that is not listed.
- `files` (optional): files the command reads, written to its working
  directory before it runs: the code of a function, a file to upload.
- `expectedExit`, `source` (the Go file that renders it), `group`, `title`,
  `description`.

Use demo IDs, example.com URLs and fixed timestamps.

## Text for people, JSON for machines

Text output is free to change; JSON output is what scripts and agents read.
`internal/cmd/views.go` holds the designed text views, chosen by the type of
the value a command prints (`render`); a type without one gets the generic
labelled output of `internal/output/generic.go`. Neither touches `-o json`.
A command that changes something prints one line saying what it did
(`printDone`, or `renderDone` when the changed item follows), and every view
ends with a `Next:` line. The layout rules are at the top of
`internal/cmd/render.go`.

The fixture test records the JSON fields of every scenario in
`testdata/json_shapes.json` and fails when one disappears. After a deliberate
change:

```sh
UPDATE_JSON_SHAPES=1 go test ./playground -run TestRunnerRendersFixtures
```

## Validate

```sh
go test ./playground
```
