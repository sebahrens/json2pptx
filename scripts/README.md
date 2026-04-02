# Scripts

Utility scripts for the Go Slide Creator project.

## Development Loop

### loop.sh

Automated development loop runner.

```bash
./scripts/loop.sh           # Run build loop
./scripts/loop.sh plan      # Run planning loop
./scripts/loop.sh 5         # Run build loop for max 5 iterations
./scripts/loop.sh plan 3    # Run planning loop for max 3 iterations
```

## Testing

### e2e_visual_test.sh

End-to-end visual testing against all templates.

```bash
TEST_MODE=all ./scripts/e2e_visual_test.sh
```

### run_tests.sh

Run the Go test suite with coverage reporting.

## Utilities

### check-licenses.sh

Check third-party license compliance.

### check_lines.sh

Count lines in the beads issues file (for monitoring issue database size).
