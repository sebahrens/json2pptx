# d-cli: the CLI only, with the installed skill

You have a shell, the `json2pptx` binary on PATH and the generate-deck skill as `json2pptx skill install` writes it. No MCP server.

## Do

1. Install the skill: `json2pptx skill install --dest $J/d-cli/skill` and read what it installed. That directory, `json2pptx --help` and each subcommand's `--help` are your only documentation.
2. Build a 7-slide deck from this brief on `forest-green`: "Pitch for a regional bakery chain to adopt our route-planning software: the problem (late deliveries, 9% of orders), what we do, three customer results with numbers, pricing in three tiers, a 90-day rollout, the team, the ask."
3. Take the path the CLI's own `get-started` recommends, then note every place the installed skill says something different.
4. Validate, render, produce slide images as files, and look at them.
5. Audit: for the commands you used, check that `--help` exits 0 on stdout, that stdout is JSON and only JSON when you ask for JSON, that exit codes follow the reported verdict, and that input/output flags mean the same thing across subcommands. Log each call's command line, exit code, stdout and stderr sizes under `$J/d-cli/log/`.

## Measure

- Commands and bytes on the happy path to the first rendered deck, separately from the audit.
- Bytes of discovery output you had to read to learn the slide kinds, the template names and one kind's fields.
- Every disagreement between the skill, the CLI help and the CLI's behaviour.
