#!/usr/bin/env python3
"""Replay a preserved benchmark input. Output approval requires fresh PNG review."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('spec', type=Path)
parser.add_argument('--binary', default='json2pptx')
parser.add_argument('--templates-dir', type=Path, default=Path('templates'))
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parent
spec = json.loads(args.spec.read_text())

def resolve(value):
    if isinstance(value, dict):
        return {key: resolve(item) for key, item in value.items()}
    if isinstance(value, list):
        return [resolve(item) for item in value]
    if isinstance(value, str) and value.startswith('@benchmark/'):
        return str(root / value.removeprefix('@benchmark/'))
    return value

with tempfile.TemporaryDirectory(prefix='consulting-replay-') as temp:
    resolved = Path(temp) / 'spec.json'
    resolved.write_text(json.dumps(resolve(spec)))
    mode = ['semantic', 'render', '--spec'] if 'meta' in spec else ['generate', '--json']
    command = [args.binary, *mode, str(resolved), '--templates-dir', str(args.templates_dir.resolve()), '--output', str(args.output.resolve())]
    raise SystemExit(subprocess.run(command, check=False).returncode)
