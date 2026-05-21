# JSON Slide Format

This page used to carry a full tutorial. The input format now has one canonical
tutorial and one canonical schema, so this stub just points at them — the
contract lives in a single place and cannot drift between two copies.

## Start here

- **Tutorial with worked examples** → [docs/INPUT_FORMAT.md](docs/INPUT_FORMAT.md).
  It walks through the top-level object, slide and content shapes, `shape_grid`,
  charts, diagrams, tables, the icon/emoji policy, and patch operations.
- **Canonical schema** → `get_input_schema` (MCP) or `json2pptx input-schema`
  (CLI). Both are generated from the live Go input structs in
  `cmd/json2pptx/json_schema.go` and are the single source of truth for fields,
  types, enums, and required-vs-optional flags. Treat the schema as the contract.

## Generate a deck

```bash
json2pptx generate -json deck.json -template midnight-blue \
  -templates-dir templates -output /tmp/out
```

## Related references

- [docs/PATTERNS.md](docs/PATTERNS.md) — named-pattern authoring guide.
- [docs/FIT_FINDINGS.md](docs/FIT_FINDINGS.md) — finding-code catalogue with
  severity and action semantics.
- [skills/generate-deck/](skills/generate-deck/) — agent-facing skill bundle with
  workflow, rules, and pattern guidance.
