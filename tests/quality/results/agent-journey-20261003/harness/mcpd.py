#!/usr/bin/env python3
"""Tiny stdio-MCP bridge: `mcpd.py serve <sock> <logdir> -- <server cmd...>` keeps one MCP session open;
`mcpd.py call <sock> <tool> '<json args>'` calls a tool; `mcpd.py list <sock>` lists tools; `mcpd.py raw <sock> <method> '<params>'`."""
import json, os, socket, subprocess, sys, threading, time

def serve(sock_path, logdir, cmd):
    os.makedirs(logdir, exist_ok=True)
    p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=open(os.path.join(logdir, "server.err"), "w"), text=True, bufsize=1)
    lock = threading.Lock(); nid = [0]
    def rpc(method, params=None, notify=False):
        with lock:
            msg = {"jsonrpc": "2.0", "method": method}
            if params is not None: msg["params"] = params
            if not notify:
                nid[0] += 1; msg["id"] = nid[0]
            p.stdin.write(json.dumps(msg) + "\n"); p.stdin.flush()
            if notify: return None
            while True:
                line = p.stdout.readline()
                if not line: return {"error": "server closed"}
                try: r = json.loads(line)
                except Exception: continue
                if r.get("id") == nid[0]: return r
    init = rpc("initialize", {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "journey", "version": "1"}})
    rpc("notifications/initialized", {}, notify=True)
    json.dump(init, open(os.path.join(logdir, "initialize.json"), "w"), indent=1)
    if os.path.exists(sock_path): os.remove(sock_path)
    s = socket.socket(socket.AF_UNIX); s.bind(sock_path); s.listen(4)
    n = 0
    while True:
        c, _ = s.accept()
        data = b""
        while not data.endswith(b"\n"):
            chunk = c.recv(65536)
            if not chunk: break
            data += chunk
        req = json.loads(data)
        t0 = time.time(); resp = rpc(req["method"], req.get("params")); dt = time.time() - t0
        n += 1
        out = json.dumps(resp)
        with open(os.path.join(logdir, "calls.jsonl"), "a") as f:
            f.write(json.dumps({"n": n, "method": req["method"], "params": req.get("params"), "seconds": round(dt, 2), "response_bytes": len(out), "is_error": bool((resp.get("result") or {}).get("isError")) or "error" in resp}) + "\n")
        with open(os.path.join(logdir, f"resp-{n:03d}.json"), "w") as f: f.write(out)
        c.sendall(out.encode() + b"\n"); c.close()

def client(sock_path, method, params):
    c = socket.socket(socket.AF_UNIX); c.connect(sock_path)
    c.sendall(json.dumps({"method": method, "params": params}).encode() + b"\n")
    data = b""
    while True:
        chunk = c.recv(1 << 20)
        if not chunk: break
        data += chunk
    return json.loads(data)

if __name__ == "__main__":
    mode = sys.argv[1]
    if mode == "serve":
        i = sys.argv.index("--"); serve(sys.argv[2], sys.argv[3], sys.argv[i+1:])
    elif mode == "list":
        r = client(sys.argv[2], "tools/list", {})
        for t in r["result"]["tools"]: print(t["name"], "-", (t.get("description") or "")[:110].replace("\n", " "))
    elif mode == "call":
        args = {}
        if len(sys.argv) > 4:
            args = json.load(open(sys.argv[4][1:])) if sys.argv[4].startswith("@") else json.loads(sys.argv[4])
        r = client(sys.argv[2], "tools/call", {"name": sys.argv[3], "arguments": args})
        res = r.get("result") or r
        sc = res.get("structuredContent")
        if sc is not None: print(json.dumps(sc, indent=1))
        for c in ([x for x in res.get("content", []) if x.get("type") != "text"] if sc is not None else res.get("content", [])):
            if True:
                if c.get("type") == "text": print(c["text"])
                elif c.get("type") == "image" and c.get("data"):
                    import base64, time as _t
                    d = os.path.join(os.path.dirname(os.path.abspath(sys.argv[2])), "images"); os.makedirs(d, exist_ok=True)
                    ext = (c.get("mimeType") or "image/jpeg").split("/")[-1]
                    fp = os.path.join(d, "img-%d-%d.%s" % (int(_t.time()*1000), res["content"].index(c), ext))
                    open(fp, "wb").write(base64.b64decode(c["data"])); print("[image saved: %s]" % fp)
                else: print("[%s content, %d bytes]" % (c.get("type"), len(json.dumps(c))))
        if res.get("isError"): print("\n[isError=true]")
    elif mode == "raw":
        print(json.dumps(client(sys.argv[2], sys.argv[3], json.loads(sys.argv[4]) if len(sys.argv) > 4 else {}), indent=1))
