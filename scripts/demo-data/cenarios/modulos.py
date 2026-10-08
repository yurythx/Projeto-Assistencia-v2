import hashlib, subprocess, time
RUN = int(time.time())
from nxlib import *  # noqa: F403

PW = open(os.path.join(ROOT, ".env")).read().split("DEMO_USER_PASSWORD=")[1].split("\n")[0]
pickr = lambda g: (lambda l: l[RUN % len(l)])(users_in(g))
a = pickr("SEMSA-UPA-ADM"); b = pickr("SEMED-ESCOLA-MARIA-ELZA-DIR"); c = pickr("SEMPRAS-CREAS-ATD")
ti = users_in("PREF-SEDE-TI")[3]
uid = lambda u: call(u, "GET", "me")[1]["data"]["id"]
A, B, C = uid(a), uid(b), uid(c)
print(f"a={a} b={b} c={c} ti={ti}")

print("\n== Signum (reautenticação pelo Keycloak)")
doc = "Contrato fictício de teste".encode()
sha = hashlib.sha256(doc).hexdigest()
code, d = call(a, "POST", "signum/envelopes", {"title": "Termo de teste", "document_sha256": sha, "signer_ids": [B, C], "sequential": False})
expect("abre envelope", code, (200, 201), d)
if code in (200, 201):
    eid = d["data"]["id"]
    code, ch = call(b, "POST", f"signum/envelopes/{eid}/challenge")
    expect("signatário pede desafio", code, (200, 201), ch)
    if code in (200, 201):
        print("   desafio:", {k: (v if k != "nonce" else "…") for k, v in ch["data"].items()})
        body = {"challenge_id": ch["data"].get("id") or ch["data"].get("challenge_id"), "nonce": ch["data"].get("nonce"), "password": "senha-errada", "confirm_document_sha256": sha}
        code, d = call(b, "POST", f"signum/envelopes/{eid}/sign", body)
        expect("senha errada é recusada", code, (401, 403, 422), d)
        code, ch = call(b, "POST", f"signum/envelopes/{eid}/challenge")
        body.update(challenge_id=ch["data"].get("id") or ch["data"].get("challenge_id"), nonce=ch["data"].get("nonce"), password=PW)
        code, d = call(b, "POST", f"signum/envelopes/{eid}/sign", body)
        expect("assina com a senha do Keycloak", code, (200, 201), d)
    code, d = call(ti, "GET", f"signum/envelopes/{eid}")
    expect("terceiro não vê o envelope", code, (403, 404))

print("\n== Mercúrio")
code, d = call(a, "POST", "mercurio/direct", {"user_id": B})
expect("abre conversa direta", code, (200, 201), d)
if code in (200, 201):
    rid = d["data"]["id"]
    code, d = call(a, "POST", f"mercurio/rooms/{rid}/messages", {"body": "Olá, teste do Mercúrio."})
    expect("envia mensagem", code, (200, 201), d)
    code, d = call(b, "GET", f"mercurio/rooms/{rid}/messages")
    expect("destinatário lê", code, 200, d)
    code, _ = call(b, "POST", f"mercurio/rooms/{rid}/read")
    expect("marcar como lida (204 pelo BFF)", code, 204)
    code, _ = call(c, "GET", f"mercurio/rooms/{rid}/messages")
    expect("terceiro não lê a conversa", code, (403, 404))

print("\n== Agenda")
code, d = call(a, "POST", "calendar/events", {"title": "Reunião de teste", "starts_at": "2026-10-05T13:00:00Z", "ends_at": "2026-10-05T14:00:00Z", "visibility": "internal"})
expect("cria evento interno", code, (200, 201), d)
code, d = call(a, "POST", "calendar/events", {"title": "Evento invertido", "starts_at": "2026-10-05T15:00:00Z", "ends_at": "2026-10-05T14:00:00Z"})
expect("fim antes do início é recusado", code, (400, 422))

print("\n== Arquivos (upload real no MinIO)")
code, d = call(a, "GET", "files/browse")
expect("navega raiz", code, 200, d)
code, d = call(a, "POST", "files/folders", {"name": f"Pasta de teste {RUN}"})
expect("cria pasta", code, (200, 201), d)
if code in (200, 201):
    fid = d["data"]["id"]
    code, t = call(a, "POST", "files/uploads", {"folder_id": fid, "filename": "teste.txt", "content_type": "text/plain", "size": len(doc)})
    expect("pede URL de upload", code, (200, 201), t)
    if code in (200, 201):
        tk = t["data"]; up = tk.get("upload") or tk
        print("   upload_url host:", up["upload_url"].split("/")[2])
        hdr = sum((["-H", f"{k}: {v}"] for k, v in (up.get("headers") or {}).items()), [])
        r = subprocess.run(["curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "-X", up.get("method") or "PUT", *hdr, "--data-binary", doc.decode(), up["upload_url"]], capture_output=True, text=True).stdout
        expect("PUT no MinIO pela URL pré-assinada", r, ("200",))
        oid = tk["file"]["id"]
        code, d = call(a, "POST", f"files/objects/{oid}/confirm")
        expect("confirma upload", code, (200, 201, 204), d)
        code, d = call(a, "GET", f"files/objects/{oid}/download")
        expect("URL de download", code, 200, d)
        if code == 200:
            url = d["data"].get("url") or d["data"].get("download_url")
            got = subprocess.run(["curl", "-s", url], capture_output=True, text=True).stdout
            check("conteúdo baixado confere", got == doc.decode(), got[:200])
        code, _ = call(c, "GET", f"files/objects/{oid}/download")
        expect("usuário de outra secretaria não baixa", code, (403, 404))

print("\n== Diretório / IAM / Busca")
code, d = call(c, "GET", "directory/people?q=silva")
expect("busca pessoas no diretório", code, 200)
if code == 200: check("encontra resultados", len(d["data"]) > 0, d)
code, d = call(c, "GET", "directory/sectors")
expect("lista setores", code, 200)
code, d = call(c, "GET", "directory/me")
expect("meu perfil do diretório", code, 200, d)
if code == 200: check("cargo pré-carregado", bool(d["data"].get("job_title")), d["data"])
code, _ = call(c, "GET", "users")
expect("servidor não lista usuários (IAM)", code, 403)
code, d = call("teste.iam", "GET", "users?q=martins")
expect("gestor IAM lista usuários", code, 200, d)
code, d = call("teste.iam", "GET", "iam/org-tree")
expect("árvore organizacional", code, 200)
if code == 200: check("as 4 entidades da prefeitura na árvore", {"PREF", "SEMSA", "SEMED", "SEMPRAS"} <= {e.get("sigla") for e in d["data"]}, [e.get("sigla") for e in d["data"]])
code, d = call(a, "GET", "search?q=teste")
expect("busca global", code, 200, d)

print("\n== Atlas")
code, d = call(a, "GET", "atlas/workflows?q=pregao")
expect("consulta procedimentos", code, 200, d)
if code == 200: check("pregão semeado na consulta (busca sem acento)", any(w["codigo_processual"] == "ADM.LIC.001" for w in d["data"]), d["data"])
code, d = call(a, "GET", "atlas/ttdd?q=2.0.02")
expect("consulta a Tabela de Temporalidade", code, 200, d)
pergunta = {"query": "Quais documentos do pregão eletrônico?"}
code, d = call(a, "POST", "atlas/chat", pergunta)
expect("assistente exige atlas:read", code, 403, d)
code, d = call("teste.admin", "POST", "atlas/chat", pergunta)
expect("assistente responde", code, 200, d)
if code == 200: check("resposta fundamentada no ADM.LIC.001", not d["data"]["refused"] and d["data"]["sources"][0]["codigo_processual"] == "ADM.LIC.001", d["data"])
code, d = call("teste.admin", "POST", "atlas/chat", {"query": "licença para viagem internacional de férias"})
expect("assistente aceita a pergunta", code, 200, d)
if code == 200: check("pergunta sem procedimento homologado é recusada", d["data"]["refused"], d["data"])
code, _ = call(a, "POST", "atlas/admin/workflows", {})
expect("servidor não cadastra procedimento", code, 403)

print("\n== Auditoria / LGPD")
code, _ = call(a, "GET", "audit/logs")
expect("servidor não vê auditoria", code, 403)
code, d = call("teste.auditor", "GET", "audit/verify")
expect("auditor verifica cadeia", code, 200, d)
if code == 200: print("   verify:", str(d["data"])[:200])
code, d = call(b, "GET", "lgpd/status")
expect("status LGPD", code, 200, d)
code, d = call(b, "POST", "lgpd/accept", {"term_version": "v1.0.0-2026"})
expect("aceite LGPD autenticado", code, (200, 201, 204), d)
done()
