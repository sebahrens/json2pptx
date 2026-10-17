#!/usr/bin/env python3
"""Focused real-engine smoke: no network, Go build, or source mutations."""
import json
import os
from pathlib import Path
import select
import shutil
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parent.parent
ENGINE = os.environ.get("PWC_LLM_WIKI_BIN", "pwc-llm-wiki")


def run():
    with tempfile.TemporaryDirectory(prefix="json2pptx-wiki-smoke-") as temporary:
        state = Path(temporary) / "state"
        environment = os.environ | {
            "PWC_LLM_WIKI_BIN": ENGINE,
            "JSON2PPTX_WIKI_STATE_DIR": str(state),
            "JSON2PPTX_WIKI_REPO": str(ROOT),
        }
        with tempfile.TemporaryFile(mode="w+") as errors:
            process = subprocess.Popen(
                [str(ROOT / "scripts/wiki.sh"), "serve"],
                stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                stderr=errors, text=True, env=environment,
            )

            def request(identifier, method, parameters):
                process.stdin.write(json.dumps({
                    "jsonrpc": "2.0", "id": identifier,
                    "method": method, "params": parameters,
                }) + "\n")
                process.stdin.flush()
                deadline = time.monotonic() + 20
                while time.monotonic() < deadline:
                    if select.select([process.stdout], [], [], 1)[0]:
                        line = process.stdout.readline()
                        if not line:
                            errors.seek(0)
                            raise AssertionError(errors.read())
                        response = json.loads(line)
                        if response.get("id") == identifier:
                            assert "error" not in response, response
                            return response["result"]
                raise AssertionError("MCP request timed out")

            try:
                initialized = request(1, "initialize", {
                    "protocolVersion": "2024-11-05", "capabilities": {},
                    "clientInfo": {"name": "json2pptx-wiki-smoke", "version": "1"},
                })
                assert initialized["serverInfo"]["version"] == "0.5.0"
                process.stdin.write('{"jsonrpc":"2.0","method":"notifications/initialized"}\n')
                process.stdin.flush()
                tools = request(2, "tools/list", {})["tools"]
                names = {tool["name"] for tool in tools}
                checks = [
                    ("wiki_stats", {}),
                    ("wiki_search", {"query": "process regions", "top_k": 2}),
                    ("wiki_list", {"type": "section", "page_size": 10}),
                    ("wiki_content_read", {"uri": "guides"}),
                    ("wiki_config", {"action": "get", "key": "ingest.auto_commit"}),
                    ("wiki_schema", {"action": "list"}),
                    ("wiki_ingest", {"path": ".", "full": True, "dry_run": True}),
                ]
                for identifier, (name, arguments) in enumerate(checks, 3):
                    assert name in names, name
                    result = request(identifier, "tools/call", {"name": name, "arguments": arguments})
                    assert not result.get("isError"), result
                    text = result["content"][0]["text"]
                    if name == "wiki_config":
                        assert "false" in text, text
                    if name == "wiki_stats":
                        stats = json.loads(text)
                        assert stats["pages"] >= 18 and stats["sections"] >= 2, stats
                    if name == "wiki_search":
                        assert json.loads(text)["results"], text
                    if name == "wiki_ingest":
                        report = json.loads(text)
                        assert report["pages_validated"] >= 18 and not report["warnings"] and not report["commit"], report
                    print(name, "PASS")
            finally:
                process.stdin.close()
                try:
                    process.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
        repository = state / "repository"
        assert not subprocess.run(
            ["git", "-C", str(repository), "rev-parse", "--verify", "HEAD"],
            capture_output=True,
        ).returncode == 0, "bootstrap created an unexpected commit"
        retained = repository / "wiki" / "runtime-only.md"
        retained.write_text('---\ntitle: Runtime edit\ntype: documentation\nstatus: active\n---\n\n## Local content\n')
        subprocess.run([str(ROOT / "scripts/wiki.sh"), "init"], env=environment, check=True, capture_output=True)
        assert retained.exists(), "restart overwrote runtime content"
        assert not (ROOT / "docs/wiki/runtime-only.md").exists(), "runtime edit leaked into source"
        print("runtime persistence/no writeback/no commits PASS")
        bundle = Path(temporary) / "bundle"
        (bundle / "docs").mkdir(parents=True)
        shutil.copytree(ROOT / "docs/wiki", bundle / "docs/wiki")
        shutil.copytree(ROOT / "schemas", bundle / "schemas")
        shutil.copyfile(ROOT / "wiki.toml", bundle / "wiki.toml")
        with (bundle / "docs/wiki/README.md").open("a") as upgraded:
            upgraded.write("\nSource bundle upgrade smoke.\n")
        previous = (repository / "wiki/README.md").read_bytes()
        upgraded_environment = environment | {"JSON2PPTX_WIKI_REPO": str(bundle)}
        upgraded = subprocess.run([str(ROOT / "scripts/wiki.sh"), "init"], env=upgraded_environment, capture_output=True, text=True, check=True)
        assert "Bundled wiki source changed" in upgraded.stderr, upgraded.stderr
        assert (repository / "wiki/README.md").read_bytes() == previous
        assert retained.exists(), "upgrade warning lost local content"
        print("changed bundle warns without overwrite PASS")

        fake_bin = Path(temporary) / "fake-bin"
        fake_bin.mkdir()
        failing_copy = fake_bin / "cp"
        failing_copy.write_text("#!/bin/sh\nexit 91\n")
        failing_copy.chmod(0o755)
        interrupted_state = Path(temporary) / "interrupted-state"
        interrupted_environment = environment | {"JSON2PPTX_WIKI_STATE_DIR": str(interrupted_state)}
        failed = subprocess.run([str(ROOT / "scripts/wiki.sh"), "init"], env=interrupted_environment | {"PATH": str(fake_bin) + os.pathsep + os.environ["PATH"]}, capture_output=True)
        assert failed.returncode != 0, "simulated copy failure was ignored"
        assert not (interrupted_state / "repository").exists(), "incomplete seed was published"
        assert not (interrupted_state / ".seed-lock").exists(), "failed seed left its lock"
        subprocess.run([str(ROOT / "scripts/wiki.sh"), "init"], env=interrupted_environment, capture_output=True, check=True)
        assert (interrupted_state / "repository/.json2pptx-wiki-ready").exists()
        print("interrupted seed remains unpublished and retry succeeds PASS")

        packaged = Path(temporary) / "installed/share/json2pptx"
        stage_command = ["sh", str(ROOT / "scripts/stage-wiki.sh"), str(packaged)]
        subprocess.run(stage_command, check=True)
        removed_page = packaged / "docs/wiki/removed-in-next-release.md"
        removed_schema = packaged / "schemas/removed-in-next-release.json"
        removed_page.write_text("old packaged page")
        removed_schema.write_text("{}")
        subprocess.run(stage_command, check=True)
        assert not removed_page.exists() and not removed_schema.exists(), "reinstall retained deleted packaged files"
        assert retained.exists(), "packaged reinstall touched runtime edits"
        for required in ["wiki.toml", "schemas/doc.json", "docs/wiki/README.md", "scripts/wiki.sh", "Dockerfile.wiki", "deploy/README.md", "deploy/mcp.wiki.example.json"]:
            assert (packaged / required).is_file(), required
        makefile = (ROOT / "Makefile").read_text()
        for staging in ["$(DIST_STAGING)", "$(WIN_DIST_STAGING)"]:
            assert 'sh scripts/stage-wiki.sh "' + staging + '/wiki-source"' in makefile
        print("package reinstall removes stale files; both archive recipes ship wiki PASS")
        restrictive = Path(temporary) / "restrictive-source"
        subprocess.run(["sh", "-c", 'umask 077; exec sh "$1" "$2"', "wiki-mode-test", str(ROOT / "scripts/stage-wiki.sh"), str(restrictive)], check=True)
        for path in [restrictive, *restrictive.rglob("*")]:
            mode = path.stat().st_mode & 0o777
            expected = 0o755 if path.is_dir() or path.suffix == ".sh" else 0o644
            assert mode == expected, (str(path), oct(mode), oct(expected))
        print("restrictive umask publishes public bundle modes PASS")
        private_prefix = Path(temporary) / "private-prefix"
        for name in ["bin", "share"]:
            directory = private_prefix / name
            directory.mkdir(parents=True, exist_ok=True)
            directory.chmod(0o700)
        subprocess.run(["make", "install-wiki", "PREFIX=" + str(private_prefix)], cwd=ROOT, check=True, capture_output=True)
        for name in ["bin", "share"]:
            assert (private_prefix / name).stat().st_mode & 0o777 == 0o700, "existing private directory was relaxed"
        public_prefix = Path(temporary) / "new-prefix"
        subprocess.run(["sh", "-c", 'umask 077; exec make install-wiki "PREFIX=$1"', "wiki-install-mode-test", str(public_prefix)], cwd=ROOT, check=True, capture_output=True)
        for name in ["bin", "share"]:
            assert (public_prefix / name).stat().st_mode & 0o777 == 0o755, "new public directory inherited restrictive umask"
        print("existing private bin/share retained; new bin/share0755 PASS")
        guard_source = Path(temporary) / "guard-source"
        shutil.copytree(packaged, guard_source)
        sentinel = guard_source / ".source-must-survive"
        sentinel.write_text("canonical source")
        source_alias = Path(temporary) / "source-alias"
        source_alias.symlink_to(guard_source, target_is_directory=True)
        parent_alias = Path(temporary) / "parent-alias"
        parent_alias.symlink_to(guard_source.parent, target_is_directory=True)
        for source, destination in [(source_alias, guard_source), (guard_source, parent_alias / guard_source.name)]:
            refused = subprocess.run(["sh", str(ROOT / "scripts/stage-wiki.sh"), str(destination)], env=os.environ | {"JSON2PPTX_WIKI_REPO": str(source)}, capture_output=True, text=True)
            assert refused.returncode != 0 and "Cannot replace the source" in refused.stderr, refused.stderr
            assert sentinel.read_text() == "canonical source", "aliased source was replaced"
        print("physical source/destination aliases refuse source replacement PASS")


def run_http():
    import secrets
    import socket
    import urllib.error
    import urllib.request

    with tempfile.TemporaryDirectory(prefix="json2pptx-wiki-http-") as temporary:
        token = secrets.token_urlsafe(32)
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            port = listener.getsockname()[1]
        environment = os.environ | {
            "PWC_LLM_WIKI_BIN": ENGINE,
            "JSON2PPTX_WIKI_REPO": str(ROOT),
            "JSON2PPTX_WIKI_STATE_DIR": temporary + "/state",
            "JSON2PPTX_WIKI_HTTP_BIND": "0.0.0.0",
            "JSON2PPTX_WIKI_HTTP_TOKEN": token,
        }
        with tempfile.TemporaryFile() as errors:
            process = subprocess.Popen(
                [str(ROOT / "scripts/wiki.sh"), "serve", "--http", str(port)],
                stdin=subprocess.PIPE, stdout=subprocess.DEVNULL,
                stderr=errors, env=environment,
            )

            def call(payload, authorized=True, session=None):
                headers = {"Content-Type": "application/json", "Accept": "application/json, text/event-stream", "Host": "localhost:" + str(port)}
                if authorized:
                    headers["Authorization"] = "Bearer " + token
                if session:
                    headers["Mcp-Session-Id"] = session
                request = urllib.request.Request("http://127.0.0.1:" + str(port) + "/mcp", data=json.dumps(payload).encode(), headers=headers)
                with urllib.request.urlopen(request, timeout=5) as response:
                    raw = response.read().decode()
                    result = None
                    if raw.lstrip().startswith("{"):
                        result = json.loads(raw)
                    else:
                        for line in raw.splitlines():
                            if line.startswith("data:") and line[5:].strip():
                                result = json.loads(line[5:].strip())
                                break
                    return response.status, response.headers, result

            initialize = {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "json2pptx-http-smoke", "version": "1"}}}
            try:
                deadline = time.monotonic() + 20
                while True:
                    try:
                        call(initialize, False)
                        raise AssertionError("unauthorized request accepted")
                    except urllib.error.HTTPError as error:
                        assert error.code == 401, error.code
                        print("HTTP unauthorized POST /mcp:401 PASS")
                        break
                    except urllib.error.URLError:
                        if time.monotonic() > deadline:
                            raise
                        time.sleep(.2)
                status, headers, result = call(initialize)
                assert status == 200 and result["result"]["serverInfo"]["version"] == "0.5.0", result
                session = headers.get("Mcp-Session-Id")
                print("HTTP initialize with Host localhost:port PASS")
                status, _, _ = call({"jsonrpc": "2.0", "method": "notifications/initialized"}, session=session)
                assert status in (200, 202, 204), status
                status, _, result = call({"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "wiki_search", "arguments": {"query": "process regions", "top_k": 2}}}, session=session)
                assert status == 200 and not result["result"].get("isError"), result
                assert json.loads(result["result"]["content"][0]["text"])["results"]
                print("HTTP authenticated wiki_search /mcp PASS")
            finally:
                process.stdin.close()
                process.terminate()
                try:
                    process.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == "__main__":
    import sys
    run()
    if "--http" in sys.argv:
        run_http()
