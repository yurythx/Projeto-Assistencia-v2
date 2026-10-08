# Utilitários dos cenários de teste (ver docs/DEPLOY.md, "Cenários").
import csv, json, os, ssl, subprocess, sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
CSV = os.path.join(ROOT, "deploy", "demo", "usuarios.csv")
NX = os.path.join(os.path.dirname(os.path.abspath(__file__)), "nx")
# HTTPS com a CA interna (scripts/enable-https.sh): curl e urllib confiam nela.
CAFILE = os.path.join(ROOT, "secrets", "ca", "nexus-ca.crt")
if os.path.exists(CAFILE):
    os.environ["CURL_CA_BUNDLE"] = CAFILE
SSL_CTX = ssl.create_default_context(cafile=CAFILE if os.path.exists(CAFILE) else None)
USERS = list(csv.DictReader(open(CSV, encoding="utf-8-sig"), delimiter=";"))
FAILS = []

def call(user, method, path, body=None):
    args = [NX, user, method, path] + ([json.dumps(body)] if body is not None else [])
    out = subprocess.run(args, capture_output=True, text=True).stdout
    first, _, rest = out.partition("\n")
    code = int(first.split()[1]) if first.startswith("HTTP") else 0
    try:
        data = json.loads(rest) if rest.strip() else None
    except json.JSONDecodeError:
        data = rest
    return code, data

def users_in(group):
    return [u["usuario"] for u in USERS if group in u["grupos"].split()]

def check(label, cond, detail=""):
    print(("  OK   " if cond else "  FALHA") + f" {label}" + ("" if cond else f"  -> {str(detail)[:400]}"))
    if not cond:
        FAILS.append(label)

def expect(label, got, want, data=None):
    check(f"{label} [{got}]", got == want if isinstance(want, int) else got in want, data)

def done():
    print(f"\n{len(FAILS)} falha(s)" + (": " + "; ".join(FAILS) if FAILS else ""))
    sys.exit(1 if FAILS else 0)


def env(key):
    for line in open(os.path.join(ROOT, ".env"), encoding="utf-8"):
        if line.startswith(key + "="):
            return line.split("=", 1)[1].strip()
    return ""


class WS:
    """Cliente WebSocket mínimo (RFC 6455, só stdlib) para os testes de
    tempo real: ticket de uso único como o usuário, Origin do frontend."""

    def __init__(self, user, origin=None):
        import base64, socket, urllib.parse
        _, d = call(user, "POST", "ws/ticket")
        u = urllib.parse.urlparse(env("WEBSOCKET_PUBLIC_URL"))
        port = u.port or (443 if u.scheme == "wss" else 80)
        raw = socket.create_connection((u.hostname, port), timeout=10)
        self.sock = SSL_CTX.wrap_socket(raw, server_hostname=u.hostname) if u.scheme == "wss" else raw
        key = base64.b64encode(os.urandom(16)).decode()
        req = (f"GET {u.path}?ticket={d['data']['ticket']} HTTP/1.1\r\nHost: {u.hostname}:{port}\r\n"
               f"Upgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: {key}\r\n"
               f"Sec-WebSocket-Version: 13\r\nOrigin: {origin or env('FRONTEND_URL')}\r\n\r\n")
        self.sock.sendall(req.encode())
        head = b""
        while b"\r\n\r\n" not in head:
            chunk = self.sock.recv(1)
            if not chunk:
                break
            head += chunk
        self.status = int(head.split(b" ")[1]) if head else 0
        self.buf = b""

    def send(self, obj):
        payload = json.dumps(obj).encode()
        mask = os.urandom(4)
        n = len(payload)
        hdr = bytes([0x81]) + (bytes([0x80 | n]) if n < 126 else bytes([0x80 | 126]) + n.to_bytes(2, "big"))
        self.sock.sendall(hdr + mask + bytes(b ^ mask[i % 4] for i, b in enumerate(payload)))

    def _read(self, n):
        while len(self.buf) < n:
            chunk = self.sock.recv(65536)
            if not chunk:
                raise ConnectionError("fechado")
            self.buf += chunk
        out, self.buf = self.buf[:n], self.buf[n:]
        return out

    def recv(self, timeout=5.0, until=None):
        """Frames JSON recebidos até `timeout` (ou até `until(frame)`)."""
        import time as _t
        frames, end = [], _t.time() + timeout
        while _t.time() < end:
            self.sock.settimeout(max(0.05, end - _t.time()))
            try:
                b0, b1 = self._read(2)
                n = b1 & 0x7F
                if n == 126:
                    n = int.from_bytes(self._read(2), "big")
                elif n == 127:
                    n = int.from_bytes(self._read(8), "big")
                data = self._read(n)
            except (TimeoutError, OSError):
                break
            op = b0 & 0x0F
            if op == 0x9:  # ping -> pong
                self.sock.sendall(bytes([0x8A, 0x80]) + os.urandom(4))
            elif op == 0x1:
                f = json.loads(data)
                frames.append(f)
                if until and until(f):
                    break
            elif op == 0x8:
                break
        return frames

    def close(self):
        try:
            self.sock.close()
        except OSError:
            pass
