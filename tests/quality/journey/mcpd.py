#!/usr/bin/env python3
"""Tiny stdio-MCP bridge for the agent-journey harness (see README.md beside it).

  mcpd.py serve <sock> <logdir> -- <server cmd...>   keep one MCP session open on a unix socket
  mcpd.py list <sock>                                tool names + first line of each description
  mcpd.py call <sock> <tool> ['<json args>'|@file]   call a tool
  mcpd.py raw <sock> <method> ['<json params>']      any JSON-RPC method, full response
  mcpd.py summary <logdir>                           the run's metrics, from <logdir>/calls.jsonl
  mcpd.py stop <sock>                                end the session and the server

`call` prints the tool's structuredContent (or its text) on stdout as JSON and everything else on
stderr: the call number, image blocks saved under <logdir>/images/, resource links, isError. Every
call is logged to <logdir>/calls.jsonl (n, ts, method, params, seconds, response_bytes, is_error)
and <logdir>/resp-NNN.json. Keep <sock> short: unix socket paths stop at about 100 characters.
"""
import base64, json, os, socket, subprocess, sys, threading, time


def serve(sock_path, logdir, cmd):
    os.makedirs(logdir, exist_ok=True)
    p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                         stderr=open(os.path.join(logdir, "server.err"), "w"), text=True, bufsize=1)
    lock = threading.Lock()
    nid = [0]

    def rpc(method, params=None, notify=False):
        with lock:
            msg = {"jsonrpc": "2.0", "method": method}
            if params is not None:
                msg["params"] = params
            if not notify:
                nid[0] += 1
                msg["id"] = nid[0]
            p.stdin.write(json.dumps(msg) + "\n")
            p.stdin.flush()
            if notify:
                return None
            while True:
                line = p.stdout.readline()
                if not line:
                    return {"error": "server closed"}
                try:
                    r = json.loads(line)
                except Exception:
                    continue
                if r.get("id") == nid[0]:
                    return r

    init = rpc("initialize", {"protocolVersion": "2025-06-18", "capabilities": {},
                              "clientInfo": {"name": "journey", "version": "1"}})
    rpc("notifications/initialized", {}, notify=True)
    with open(os.path.join(logdir, "initialize.json"), "w") as f:
        json.dump(init, f, indent=1)
    if os.path.exists(sock_path):
        os.remove(sock_path)
    s = socket.socket(socket.AF_UNIX)
    s.bind(sock_path)
    s.listen(4)
    n = 0
    while True:
        c, _ = s.accept()
        data = b""
        while not data.endswith(b"\n"):
            chunk = c.recv(65536)
            if not chunk:
                break
            data += chunk
        req = json.loads(data)
        if req["method"] == "__stop__":
            c.sendall(b"{}\n")
            c.close()
            break
        t0 = time.time()
        resp = rpc(req["method"], req.get("params"))
        dt = time.time() - t0
        n += 1
        out = json.dumps(resp)
        with open(os.path.join(logdir, "calls.jsonl"), "a") as f:
            f.write(json.dumps({
                "n": n, "ts": round(t0, 2), "method": req["method"], "params": req.get("params"),
                "seconds": round(dt, 2), "response_bytes": len(out),
                "is_error": bool((resp.get("result") or {}).get("isError")) or "error" in resp}) + "\n")
        with open(os.path.join(logdir, f"resp-{n:03d}.json"), "w") as f:
            f.write(out)
        # The bridge's own note travels beside the response, never inside what is measured.
        resp["_bridge"] = {"n": n, "seconds": round(dt, 2), "response_bytes": len(out), "logdir": os.path.abspath(logdir)}
        c.sendall(json.dumps(resp).encode() + b"\n")
        c.close()
    s.close()
    os.remove(sock_path)
    p.stdin.close()
    p.wait(timeout=10)


def client(sock_path, method, params):
    c = socket.socket(socket.AF_UNIX)
    c.connect(sock_path)
    c.sendall(json.dumps({"method": method, "params": params}).encode() + b"\n")
    data = b""
    while True:
        chunk = c.recv(1 << 20)
        if not chunk:
            break
        data += chunk
    return json.loads(data)


def note(msg):
    print(msg, file=sys.stderr)


def call(sock_path, tool, raw_args):
    args = {}
    if raw_args is not None:
        if raw_args.startswith("@"):
            with open(raw_args[1:]) as f:
                args = json.load(f)
        else:
            args = json.loads(raw_args)
    r = client(sock_path, "tools/call", {"name": tool, "arguments": args})
    bridge = r.pop("_bridge", {})
    res = r.get("result") or r
    sc = res.get("structuredContent")
    if sc is not None:
        print(json.dumps(sc, indent=1))
    n = bridge.get("n", 0)
    if bridge:
        note("[call n=%d: %d bytes, %.2fs]" % (n, bridge["response_bytes"], bridge["seconds"]))
    imgdir = os.path.join(bridge.get("logdir") or os.path.dirname(os.path.abspath(sock_path)), "images")
    for i, c in enumerate(res.get("content", [])):
        kind = c.get("type")
        if kind == "text":
            if sc is None:
                print(c["text"])
        elif kind == "image" and c.get("data"):
            os.makedirs(imgdir, exist_ok=True)
            ext = (c.get("mimeType") or "image/jpeg").split("/")[-1]
            fp = os.path.join(imgdir, "n%03d-%02d.%s" % (n, i, ext))
            with open(fp, "wb") as f:
                f.write(base64.b64decode(c["data"]))
            note("[image saved: %s]" % fp)
        elif kind == "resource_link":
            note("[resource_link: %s]" % c.get("uri"))
        else:
            note("[%s content, %d bytes]" % (kind, len(json.dumps(c))))
    if res.get("isError"):
        note("[isError=true]")
    if "error" in r:
        print(json.dumps(r["error"], indent=1))
        note("[JSON-RPC error]")


def summary(logdir):
    """The numbers a results row records, from the call log alone."""
    calls = []
    with open(os.path.join(logdir, "calls.jsonl")) as f:
        for line in f:
            if line.strip():
                calls.append(json.loads(line))
    by_tool, image_bytes = {}, 0
    validates = renders = 0
    first_ready = None
    for c in calls:
        name = (c.get("params") or {}).get("name") if c["method"] == "tools/call" else c["method"]
        by_tool[name] = by_tool.get(name, 0) + 1
        try:
            with open(os.path.join(logdir, "resp-%03d.json" % c["n"])) as f:
                res = (json.load(f).get("result") or {})
        except OSError:
            res = {}
        for block in res.get("content") or []:
            if block.get("type") == "image":
                image_bytes += len(block.get("data") or "")
        if first_ready is None and name == "validate_deck_spec":
            validates += 1
        if first_ready is None and name == "render_deck_spec":
            renders += 1
            if (res.get("structuredContent") or {}).get("deterministic_ready"):
                first_ready = c["n"]
    total = sum(c["response_bytes"] for c in calls)
    out = {
        "tool_calls": sum(1 for c in calls if c["method"] == "tools/call"),
        "total_response_bytes": total,
        "response_bytes_excluding_images": total - image_bytes,
        "errors": sum(1 for c in calls if c["is_error"]),
        "validates_before_first_ready_render": validates,
        "renders_to_first_ready": renders if first_ready is not None else None,
        "first_ready_render_call": first_ready,
        "server_seconds": round(sum(c["seconds"] for c in calls), 2),
        "by_tool": dict(sorted(by_tool.items())),
    }
    if calls and "ts" in calls[0]:
        out["wall_minutes"] = round((calls[-1]["ts"] + calls[-1]["seconds"] - calls[0]["ts"]) / 60, 1)
    print(json.dumps(out, indent=1))


def main(argv):
    if len(argv) < 3:
        sys.exit(__doc__)
    mode = argv[1]
    if mode == "serve":
        i = argv.index("--")
        serve(argv[2], argv[3], argv[i + 1:])
    elif mode == "list":
        r = client(argv[2], "tools/list", {})
        for t in r["result"]["tools"]:
            print(t["name"], "-", (t.get("description") or "")[:110].replace("\n", " "))
    elif mode == "call":
        call(argv[2], argv[3], argv[4] if len(argv) > 4 else None)
    elif mode == "raw":
        r = client(argv[2], argv[3], json.loads(argv[4]) if len(argv) > 4 else {})
        r.pop("_bridge", None)
        print(json.dumps(r, indent=1))
    elif mode == "summary":
        summary(argv[2])
    elif mode == "stop":
        client(argv[2], "__stop__", {})
    else:
        sys.exit(__doc__)


if __name__ == "__main__":
    main(sys.argv)
