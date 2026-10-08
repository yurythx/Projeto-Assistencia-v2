"""Tempo real (WebSocket), Mercúrio, Agenda, Arquivos, Diretório e Signum."""
import hashlib
import subprocess
import time
import urllib.request

from nxlib import *  # noqa: F403

RUN = int(time.time())
pickr = lambda g, k=0: (lambda l: l[(RUN + k) % len(l)])(users_in(g))  # noqa: E731
A, B, C = pickr("SEMSA-UPA-ADM"), pickr("SEMSA-UPA-ATD", 1), pickr("SEMED-ESCOLA-SILVESTRE-DIR", 2)
ADMIN, EDITOR = "teste.admin", "teste.conteudo"
PW = env("DEMO_USER_PASSWORD")
FE = env("FRONTEND_URL")
me = {u: call(u, "GET", "me")[1]["data"] for u in (A, B, C)}
uid = {u: me[u]["id"] for u in me}
dep = {u: next(s for s in me[u]["scopes"] if s.get("departamento_id")) for u in me}
print(f"A={A} (UPA/ADM)  B={B} (UPA/ATD)  C={C} (Escola Silvestre/DIR)")


def public(method, path, body=None):
    req = urllib.request.Request(f"{FE}/api/public/v1/{path}", method=method,
                                 data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=15, context=SSL_CTX) as r:
            return r.status, json.loads(r.read() or b"null")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"null")


print("\n== WebSocket: handshake e origem")
ws_bad = WS(B, origin="https://evil.example")
expect("origem estranha recusada no handshake", ws_bad.status, 403)
ws_bad.close()
wsB, wsC = WS(B), WS(C)
expect("B conecta (101)", wsB.status, 101)
expect("C conecta (101)", wsC.status, 101)

print("\n== Mercúrio: conversa direta em tempo real")
code, d = call(A, "POST", "mercurio/direct", {"user_id": uid[B]})
expect("A abre conversa com B", code, (200, 201), d)
room = d["data"]["id"]
topic = f"mercurio:room:{room}"
wsB.send({"type": "subscribe", "topic": topic})
got = wsB.recv(3, until=lambda f: f.get("type") in ("subscribed", "error"))
check("B assina a sala", any(f.get("type") == "subscribed" for f in got), got)
wsC.send({"type": "subscribe", "topic": topic})
got = wsC.recv(3, until=lambda f: f.get("type") in ("subscribed", "error"))
check("C (fora da conversa) não assina", any(f.get("type") == "error" for f in got) and not any(f.get("type") == "subscribed" for f in got), got)

code, d = call(A, "POST", f"mercurio/rooms/{room}/messages", {"body": f"Olá B, teste em tempo real {RUN}"})
expect("A envia", code, (200, 201), d)
mid = d["data"]["id"]
got = wsB.recv(5, until=lambda f: str(RUN) in json.dumps(f))
check("B recebe a mensagem pelo WebSocket", any(str(RUN) in json.dumps(f) for f in got), got)
unread_of = lambda u: next((r.get("unread") for r in call(u, "GET", "mercurio/rooms")[1]["data"] if r["id"] == room), None)  # noqa: E731
check("B tem 1 não lida", unread_of(B) == 1, unread_of(B))
code, d = call(B, "POST", f"mercurio/rooms/{room}/messages", {"body": "Recebido, obrigado!"})
expect("B responde", code, (200, 201), d)
code, d = call(A, "GET", f"mercurio/rooms/{room}/messages")
check("A vê a resposta de B no histórico", code == 200 and any("Recebido" in m.get("body", "") for m in d["data"]), d)
code, _ = call(B, "PATCH", f"mercurio/messages/{mid}", {"body": "adulterada"})
expect("B não edita mensagem de A", code, (403, 404))
code, d = call(A, "PATCH", f"mercurio/messages/{mid}", {"body": f"Olá B (editada) {RUN}"})
expect("A edita a própria", code, 200, d)
got = wsB.recv(5, until=lambda f: "editada" in json.dumps(f))
check("B recebe a edição em tempo real", any("editada" in json.dumps(f) for f in got), got)
code, _ = call(A, "DELETE", f"mercurio/messages/{mid}")
expect("A apaga a própria", code, (200, 204))
got = wsB.recv(5, until=lambda f: mid in json.dumps(f))
check("B recebe a exclusão em tempo real", any(mid in json.dumps(f) for f in got), got)
code, _ = call(A, "POST", f"mercurio/rooms/{room}/messages", {"body": "Outra mensagem"})
check("nova mensagem de A = 1 não lida para B", unread_of(B) == 1, unread_of(B))
call(B, "POST", f"mercurio/rooms/{room}/read")
check("marcar como lida zera as não lidas", unread_of(B) == 0, unread_of(B))

print("\n== Mercúrio: sala de departamento")
group = next(g for g in me[A]["groups"])
code, _ = call(A, "POST", "mercurio/rooms", {"kind": "department", "name": "indevida", "ad_group": group})
expect("servidor não cria sala", code, 403)
code, d = call(ADMIN, "POST", "mercurio/rooms", {"kind": "department", "name": f"Equipe {group} {RUN}", "ad_group": group})
expect("admin cria sala do departamento de A", code, (200, 201), d)
if code in (200, 201):
    droom = d["data"]["id"]
    code, d = call(A, "GET", "mercurio/rooms")
    check("A (do departamento) vê a sala", any(r["id"] == droom for r in d["data"]), [r["name"] for r in d["data"]])
    code, d = call(C, "GET", "mercurio/rooms")
    check("C (outra secretaria) não vê a sala", not any(r["id"] == droom for r in d["data"]))
    code, _ = call(C, "POST", f"mercurio/rooms/{droom}/messages", {"body": "intrusa"})
    expect("C não posta na sala", code, (403, 404))
    code, _ = call(A, "POST", f"mercurio/rooms/{droom}/messages", {"body": "Bom dia, equipe!"})
    expect("A posta na sala do departamento", code, (200, 201))

print("\n== Difusão geral: outbox -> RabbitMQ -> WebSocket")
code, d = call(EDITOR, "POST", "blog/posts", {"title": f"Comunicado em tempo real {RUN}", "summary": "s", "body": "b", "kind": "comunicado"})
pid = d["data"]["id"]
call(EDITOR, "POST", f"blog/posts/{pid}/publish")
got = wsC.recv(15, until=lambda f: "blog.post.published" in json.dumps(f) and pid in json.dumps(f))
check("C recebe blog.post.published pelo WebSocket", any("blog.post.published" in json.dumps(f) and pid in json.dumps(f) for f in got), [f.get("type") for f in got])

print("\n== Agenda: sala, conflito, visibilidade e público")
code, d = call(A, "POST", "calendar/rooms", {"name": "indevida"})
expect("servidor não cria sala de reunião", code, 403)
code, d = call(ADMIN, "POST", "calendar/rooms", {"name": f"Auditório {RUN}", "location": "Sede", "capacity": 80, "resources": ["projetor", "som"]})
expect("admin cria sala", code, (200, 201), d)
rid = d["data"]["id"]
day = time.strftime("%Y-%m-%d", time.gmtime(time.time() + 86400 * (7 + RUN % 20)))
ev = lambda t, s, e, **k: {"title": t, "starts_at": f"{day}T{s}:00Z", "ends_at": f"{day}T{e}:00Z", **k}  # noqa: E731
code, d = call(A, "POST", "calendar/events", ev("Reunião de planejamento", "13:00", "14:00", room_id=rid, visibility="internal"))
expect("A reserva 13h-14h", code, (200, 201), d)
e1 = d["data"]["id"]
code, d = call(C, "POST", "calendar/events", ev("Reunião concorrente", "13:30", "14:30", room_id=rid, visibility="internal"))
expect("C não reserva horário sobreposto", code, 409, d)
code, d = call(C, "POST", "calendar/events", ev("Reunião seguinte", "14:00", "15:00", room_id=rid, visibility="internal"))
expect("C reserva logo depois (14h-15h)", code, (200, 201), d)
code, d = call(B, "GET", f"calendar/rooms/{rid}/busy?from={day}T00:00:00Z&to={day}T23:59:59Z")
expect("ocupação da sala", code, 200, d)
if code == 200:
    check("2 blocos ocupados", len(d["data"]) == 2, d["data"])
code, d = call(A, "PUT", f"calendar/events/{e1}", ev("Reunião de planejamento (pauta nova)", "13:00", "13:45", room_id=rid, visibility="internal"))
expect("A altera o próprio evento", code, 200, d)
code, _ = call(C, "PUT", f"calendar/events/{e1}", ev("sequestro", "13:00", "13:45", room_id=rid))
expect("C não altera evento de A", code, (403, 404))
code, d = call(A, "POST", "calendar/events", ev("Consulta particular", "08:00", "09:00", visibility="private"))
priv = d["data"]["id"]
code, _ = call(C, "GET", f"calendar/events/{priv}")
expect("evento privado invisível para C", code, (403, 404))
code, d = call(ADMIN, "POST", "calendar/events", ev(f"Campanha de vacinação {RUN}", "12:00", "18:00", visibility="public", location="Praça central"))
expect("admin cria evento público", code, (200, 201), d)
code, d = public("GET", f"calendar/public/events?from={day}T00:00:00Z&to={day}T23:59:59Z")
check("evento público no site (anônimo)", code == 200 and any(str(RUN) in e.get("title", "") for e in d["data"]), d)
check("evento interno fora do site", code == 200 and not any(e.get("id") == e1 for e in d["data"]))
code, _ = call(A, "POST", f"calendar/events/{e1}/cancel")
expect("A cancela", code, (200, 204))
code, d = call(B, "GET", f"calendar/rooms/{rid}/busy?from={day}T00:00:00Z&to={day}T23:59:59Z")
check("cancelado libera a sala", code == 200 and len(d["data"]) == 1, d)

print("\n== Arquivos: compartilhamento por departamento")
code, d = call(A, "POST", "files/folders", {"name": f"Escalas UPA {RUN}"})
fid = d["data"]["id"]
doc = f"Escala de plantão {RUN}".encode()
code, t = call(A, "POST", "files/uploads", {"folder_id": fid, "filename": "escala.txt", "content_type": "text/plain", "size": len(doc)})
up, obj = t["data"]["upload"], t["data"]["file"]["id"]
r = subprocess.run(["curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "-X", "PUT", "-H", "Content-Type: text/plain", "--data-binary", doc.decode(), up["upload_url"]], capture_output=True, text=True).stdout
expect("upload no MinIO", r, ("200",))
call(A, "POST", f"files/objects/{obj}/confirm")
code, _ = call(B, "GET", f"files/objects/{obj}/download")
expect("B sem acesso antes do compartilhamento", code, 403)
code, d = call(A, "PUT", f"files/folders/{fid}/acl", {"entries": [{"subject_type": "departamento", "subject": dep[B]["departamento_id"], "can_write": False}]})
expect("A compartilha (leitura) com o departamento de B", code, 200, d)
code, d = call(B, "GET", f"files/objects/{obj}/download")
expect("B baixa depois do compartilhamento", code, 200, d)
code, _ = call(B, "POST", "files/uploads", {"folder_id": fid, "filename": "b.txt", "content_type": "text/plain", "size": 1})
expect("B não envia (só leitura)", code, 403)
code, _ = call(C, "GET", f"files/objects/{obj}/download")
expect("C (fora do departamento) segue sem acesso", code, 403)
code, d = call(A, "PATCH", f"files/objects/{obj}", {"name": "escala-outubro.txt", "folder_id": fid})
expect("A renomeia o arquivo", code, 200, d)
code, d = call(A, "PATCH", f"files/folders/{fid}", {"name": f"Escalas UPA 2026 {RUN}"})
expect("A renomeia a pasta", code, 200, d)
code, _ = call(B, "DELETE", f"files/objects/{obj}")
expect("B não apaga", code, 403)
code, _ = call(A, "DELETE", f"files/objects/{obj}")
expect("A apaga o arquivo", code, (200, 204))

print("\n== Diretório")
code, d = call(A, "PUT", "directory/me", {"job_title": me[A].get("name") and "Chefe Administrativo(a)", "phone": "(00) 0000-0000", "extension": "2999", "bio": f"Plantão administrativo {RUN}"})
expect("A atualiza o próprio perfil", code, 200, d)
code, d = call(B, "GET", f"directory/people/{uid[A]}")
check("B vê o perfil atualizado de A", code == 200 and str(RUN) in json.dumps(d), d)
code, _ = call(B, "PUT", f"directory/people/{uid[A]}", {"job_title": "hack"})
expect("B não edita perfil de A", code, 403)
code, d = public("GET", "directory/public/sectors")
check("setores públicos (anônimo)", code == 200 and len(d["data"]) > 0, code)

print("\n== Signum: sequencial, recusa, cancelamento e verificação pública")
sha = hashlib.sha256(f"Ata {RUN}".encode()).hexdigest()


def sign(user, env_id, pw=PW):
    c, ch = call(user, "POST", f"signum/envelopes/{env_id}/challenge")
    if c not in (200, 201):
        return c, ch
    return call(user, "POST", f"signum/envelopes/{env_id}/sign", {"challenge_id": ch["data"]["challenge_id"], "nonce": ch["data"]["nonce"], "password": pw, "confirm_document_sha256": sha})


code, d = call(A, "POST", "signum/envelopes", {"title": f"Ata sequencial {RUN}", "document_sha256": sha, "signer_ids": [uid[B], uid[C]], "sequential": True})
expect("A abre envelope sequencial B -> C", code, (200, 201), d)
eid = d["data"]["id"]
code, d = sign(C, eid)
expect("C não assina antes de B", code, (403, 409, 422), d)
code, d = sign(B, eid)
expect("B assina", code, 200, d)
code, d = sign(C, eid)
expect("C assina em seguida", code, 200, d)
code, d = call(A, "GET", f"signum/envelopes/{eid}")
check("envelope concluído", code == 200 and d["data"].get("status") == "completed", d["data"].get("status") if code == 200 else d)
code, d = public("GET", f"signum/verify/{eid}")
check("verificação pública (anônimo) confirma", code == 200 and "completed" in json.dumps(d), d)
code, d = call(A, "POST", "signum/envelopes", {"title": f"Termo a recusar {RUN}", "document_sha256": sha, "signer_ids": [uid[B]]})
e2 = d["data"]["id"]
code, d = call(B, "POST", f"signum/envelopes/{e2}/refuse", {"reason": "Cláusula 3 divergente"})
expect("B recusa com motivo", code, 200, d)
code, d = call(A, "GET", f"signum/envelopes/{e2}")
check("status recusado", code == 200 and d["data"].get("status") == "refused", d["data"].get("status") if code == 200 else d)
code, d = call(A, "POST", "signum/envelopes", {"title": f"Termo a cancelar {RUN}", "document_sha256": sha, "signer_ids": [uid[C]]})
e3 = d["data"]["id"]
code, _ = call(C, "POST", f"signum/envelopes/{e3}/cancel", {"reason": "não sou o dono"})
expect("signatário não cancela", code, (403, 404))
code, d = call(A, "POST", f"signum/envelopes/{e3}/cancel", {"reason": "Documento substituído"})
expect("dono cancela", code, 200, d)
code, d = sign(C, e3)
expect("não se assina envelope cancelado", code, (403, 409, 422), d)

wsB.close()
wsC.close()
done()
