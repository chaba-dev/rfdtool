# rfdtool

`rfdtool` checks, links, starts, and reports on a project's Requests for
Discussion (RFDs) written in AsciiDoc. Run `rfdtool check` in CI to keep every
RFD's layout, metadata, lifecycle, and links valid, and the other commands
while writing.

```sh
rfdtool status            # each RFD's state and implementation progress
rfdtool check             # validate every RFD; exits 1 on problems
rfdtool link              # link every unlinked mention of another RFD
rfdtool new "Title"       # start the next RFD from the template
```

## Installing

```sh
go install github.com/chaba-dev/rfdtool@latest
```

To pin the version a repository uses, add it as a Go tool and run it with
`go tool rfdtool`:

```sh
go get -tool github.com/chaba-dev/rfdtool@latest
```

With Nix, run it with `nix run github:chaba-dev/rfdtool -- check`, or add
the flake's `packages.default` to a dev shell.

To start a project, create the RFD directory and the first RFD:

```sh
mkdir rfd
rfdtool new --labels core "First RFD"
```

## Configuring a project

`rfdtool` reads `rfd.toml` from the directory it runs in. Every setting is
optional:

```toml
# Directory of the RFDs, one NNNN directory each. Default: "rfd".
dir = "rfd"

# Template directory with README.adoc and IMPLEMENTATION.adoc that
# rfdtool new starts an RFD from. Default: the built-in template.
template = "template"

# Labels rfdtool new gives an RFD, starting with its project. rfdtool new
# requires labels, from here or from --labels.
labels = "core"

# Projects whose RFDs are private by default and must never be public.
private_projects = ["internal"]
```

`dir` and `template` set in the file are relative to the file; given as flags
or environment variables, they are relative to the working directory. Flags
override environment variables, which override the file:
`--config` or `RFD_CONFIG`, `--dir` or `RFD_DIR`, `--private-projects` or
`RFD_PRIVATE_PROJECTS`, and `new --template` and `new --labels`. An unknown
setting is an error.

## The RFD format

Each RFD is a directory `NNNN/` with a four-digit number:

- `README.adoc`: the RFD. It starts with AsciiDoc attributes and then one
  title, `= RFD N Title`, whose number matches the directory. There is no
  front matter.

  ```asciidoc
  :authors: Ada Lovelace <ada@example.com>
  :state: discussion
  :discussion: https://github.com/example/rfd/pull/12
  :labels: core, storage
  :visibility: public
  :idprefix:
  :idseparator: -

  = RFD 12 Storage layout
  ```

  `authors`, `state`, `discussion`, and `labels` each appear exactly once;
  `discussion` may be empty until the state needs it. `authors` needs a name
  and an address in angle brackets. `labels` starts with the RFD's project.
  `visibility` is optional: `public`, or `private` for RFDs only signed-in
  readers should see. RFDs of a private project default to private and cannot
  be made public. Visibility is metadata that `rfdtool check` enforces and a
  site can read; `rfdtool` itself does not publish or hide anything.

  Section ids follow `:idprefix:` empty and `:idseparator: -`, so `== Goal`
  is `<<goal>>`. The built-in template sets both; render RFDs with them, or
  `rfdtool check` accepts references that the default Asciidoctor settings
  would break.
- `IMPLEMENTATION.adoc` (or `.md` or `.org`): the implementation checklist.
  Its first line is `= RFD NNNN implementation checklist` (`#` in Markdown,
  `#+TITLE:` in Org), it links back to `README.adoc`, the README links to it,
  and its tasks are `* [ ]` and `* [x]` items. Checkboxes belong here, not in
  the README.

`state` follows the lifecycle

```
prediscussion ⇄ ideation → discussion → published → committed
                                                  ↘ abandoned
```

where an RFD may skip ahead but not move back, except between
`prediscussion` and `ideation`, and any open state may be `abandoned`.
`discussion`, `published`, and `committed` need a discussion URL, and
`committed` needs a non-empty checklist with every task checked.

`rfdtool check --base-ref REV` (or `RFD_BASE_REF`) also compares the RFDs
with a git revision, as CI should: states may only move forward, RFDs are
never deleted, and committed or abandoned RFDs are final. Relabeling,
publishing, linking mentions, rewrapping, and moving the checklist between
formats are still allowed on a final RFD. `--base-dir` compares with another
directory instead.

### Links between RFDs

Every mention of another RFD links to it with a relative source link, which
renders on GitHub and which a site can rewrite into its own pages:

```asciidoc
See link:../0005/README.adoc[RFD 5], and RFDs link:../0013/README.adoc[13]
and link:../0014/README.adoc[14].
```

`rfdtool check` rejects unlinked mentions, links to RFDs or sections that do
not exist, and links from a public RFD to a private one, in `README.adoc` and
`IMPLEMENTATION.adoc`. `rfdtool link` adds the missing links. Links in
Markdown and Org checklists are not checked.

## Developing

The flake's dev shell has Go, gopls, gotools, golangci-lint, and delve. With
[direnv](https://direnv.net/), run `direnv allow` once; otherwise enter it with
`nix develop`. Then:

```sh
go test ./...
nix flake check   # builds the package and runs its tests, as CI does
```

When `go.mod` or `go.sum` change, update `vendorHash` in `flake.nix`: set it to
`pkgs.lib.fakeHash`, run `nix build`, and copy the hash it reports.

## Output

`status` and `check` color their output on a terminal. `NO_COLOR` turns color
off and `FORCE_COLOR` turns it on.
