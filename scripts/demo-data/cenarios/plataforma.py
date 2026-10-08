"""IAM, usuários locais, módulos, branding, auditoria, transparência,
monitoramento, LGPD, configuração do Keycloak e o módulo Exemplo."""
import subprocess
import time
import urllib.request

from nxlib import *  # noqa: F403

RUN = int(time.time())
pickr = lambda g, k=0: (lambda l: l[(RUN + k) % len(l)])(users_in(g))  # noqa: E731
ADMIN, IAM, AUD = "teste.admin", "teste.iam", "teste.auditor"
S = pickr("SEMPRAS-CRAS-ANA-CARLA-ATD")
FE = env("FRONTEND_URL")
print(f"servidor={S}")


def public(method, path, body=None):
    req = urllib.request.Request(f"{FE}/api/public/v1/{path}", method=method,
                                 data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=15, context=SSL_CTX) as r:
            return r.status, json.loads(r.read() or b"null")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"null")


def local_login(username, password):
    """Login local direto na API (mesmo endpoint do NextAuth Credentials)."""
    req = urllib.request.Request(env("API_PUBLIC_URL") + "/api/v1/auth/login", method="POST",
                                 data=json.dumps({"username": username, "password": password}).encode(),
                                 headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=15, context=SSL_CTX) as r:
            return r.status
    except urllib.error.HTTPError as e:
        return e.code


print("\n== IAM: estrutura organizacional (CRUD)")
code, _ = call(S, "POST", "iam/entidades", {"nome": "indevida"})
expect("servidor não cria entidade", code, 403)
code, d = call(IAM, "POST", "iam/entidades", {"nome": f"Autarquia de Teste {RUN}", "sigla": "AUT"})
expect("gestor IAM cria entidade", code, (200, 201), d)
ent = d["data"]["id"]
code, d = call(IAM, "POST", "iam/unidades", {"entidade_id": ent, "nome": "Sede da Autarquia", "sigla": "SEDE", "email": "sede@nexus.test"})
expect("cria unidade", code, (200, 201), d)
uni = d["data"]["id"]
code, d = call(IAM, "POST", "iam/unidades", {"entidade_id": ent, "parent_id": uni, "nome": "Anexo", "sigla": "ANX"})
expect("cria unidade filha (hierarquia)", code, (200, 201), d)
filha = d["data"]["id"]
code, d = call(IAM, "POST", "iam/departamentos", {"unidade_id": uni, "nome": "Protocolo Geral", "sigla": "PG", "ad_group": f"AUT-{RUN}-PG"})
expect("cria departamento", code, (200, 201), d)
dp = d["data"]["id"]
code, d = call(IAM, "PUT", f"iam/unidades/{uni}", {"entidade_id": ent, "nome": "Sede da Autarquia (renomeada)", "sigla": "SEDE", "telefone": "0000"})
expect("renomeia unidade", code, 200, d)
code, d = call(IAM, "GET", "iam/org-tree")
check("nova entidade na árvore", code == 200 and f"Autarquia de Teste {RUN}" in json.dumps(d, ensure_ascii=False), code)

print("\n== IAM: perfil customizado + lotação manual (efeito imediato)")
code, d = call(IAM, "POST", "iam/perfis", {"nome": "Inválido", "permissoes": ["NÃO VALE"]})
expect("permissão malformada recusada", code, (400, 422), d)
code, d = call(IAM, "POST", "iam/perfis", {"nome": f"Leitor de Auditoria {RUN}", "descricao": "teste", "permissoes": ["audit:read"]})
expect("gestor IAM não concede permissão que não tem (anti-escalada)", code, 403, d)
code, d = call(ADMIN, "POST", "iam/perfis", {"nome": f"Leitor de Auditoria {RUN}", "descricao": "teste", "permissoes": ["audit:read"]})
expect("admin cria perfil customizado", code, (200, 201), d)
perfil = d["data"]["id"]
code, _ = call(S, "GET", "audit/logs")
expect("antes da lotação: servidor sem auditoria", code, 403)
sid = call(S, "GET", "me")[1]["data"]["id"]
code, d = call(IAM, "POST", f"users/{sid}/lotacoes", {"perfil_id": perfil, "entidade_id": ent, "unidade_id": uni, "departamento_id": dp})
expect("gestor IAM não lota com perfil acima do dele", code, 403, d)
code, d = call(ADMIN, "POST", f"users/{sid}/lotacoes", {"perfil_id": perfil, "entidade_id": ent, "unidade_id": uni, "departamento_id": dp})
expect("admin lota o servidor com o perfil no departamento", code, (200, 201), d)
lot_dp = d["data"]["id"]
code, d = call(S, "GET", "audit/logs?page_size=100")
expect("audit:read no departamento: consulta liberada (ADR 013)", code, 200)
check("só as ações de quem é da área (o admin global não aparece)", code == 200 and all(r.get("actor_id") != call(ADMIN, "GET", "me")[1]["data"]["id"] for r in d["data"]), code)
code, _ = call(S, "GET", "audit/verify")
expect("verificar a cadeia continua só global", code, 403)
code, _ = call(ADMIN, "DELETE", f"users/{sid}/lotacoes/{lot_dp}")
expect("remove a lotação com escopo", code, (200, 204))
code, d = call(ADMIN, "POST", f"users/{sid}/lotacoes", {"perfil_id": perfil})
expect("admin lota o servidor com o perfil (global)", code, (200, 201), d)
lot = d["data"]["id"]
code, _ = call(S, "GET", "audit/logs")
expect("depois da lotação: acesso imediato (cache invalidado)", code, 200)
code, d = call(IAM, "GET", f"users/{sid}/lotacoes")
check("lotação listada", code == 200 and any(l.get("id") == lot for l in d["data"]), d)
code, _ = call(ADMIN, "DELETE", f"users/{sid}/lotacoes/{lot}")
expect("remove a lotação", code, (200, 204))
code, _ = call(S, "GET", "audit/logs")
expect("sem a lotação: acesso revogado na hora", code, 403)

print("\n== IAM: mapeamento de grupo")
code, d = call(ADMIN, "POST", "iam/ad-mappings", {"ad_group": f"AUT-{RUN}-PG", "perfil_id": perfil, "departamento_id": dp, "unidade_id": uni, "entidade_id": ent, "descricao": "teste"})
expect("cria mapeamento grupo -> perfil", code, (200, 201), d)
mp = d["data"]["id"]
code, d = call(IAM, "GET", "iam/ad-mappings")
check("mapeamento listado", code == 200 and any(m["id"] == mp for m in d["data"]), code)
code, _ = call(ADMIN, "DELETE", f"iam/ad-mappings/{mp}")
expect("remove mapeamento", code, (200, 204))

print("\n== IAM: exclusões protegidas e limpeza")
code, d = call(IAM, "DELETE", "iam/perfis/" + next(p["id"] for p in call(IAM, "GET", "iam/perfis")[1]["data"] if p["slug"] == "servidor"))
expect("perfil de sistema não é removido", code, (403, 409, 422), d)
code, _ = call(ADMIN, "DELETE", f"iam/perfis/{perfil}")
expect("remove perfil customizado", code, (200, 204))
for kind, i in (("departamentos", dp), ("unidades", filha), ("unidades", uni), ("entidades", ent)):
    code, d = call(IAM, "DELETE", f"iam/{kind}/{i}")
    expect(f"remove {kind[:-1]} de teste", code, (200, 204), d)
code, d = call(IAM, "GET", "iam/permissions")
check("catálogo de permissões por módulo", code == 200 and len(d["data"]) >= 10, code)

print("\n== Usuários locais")
uname = f"local.teste{RUN}"
code, d = call(IAM, "POST", "users", {"username": uname, "email": f"{uname}@nexus.test", "display_name": "Local Teste", "password": "curta"})
expect("senha fraca recusada", code, (400, 422), d)
pw1 = f"Senha-forte-{RUN}!"
code, d = call(IAM, "POST", "users", {"username": uname, "email": f"{uname}@nexus.test", "display_name": "Local Teste", "password": pw1})
expect("cria usuário local", code, (200, 201), d)
lid = d["data"]["id"]
expect("login local funciona", local_login(uname, pw1), 200)
code, d = call(IAM, "PATCH", f"users/{lid}", {"active": False, "display_name": "Local Teste"})
expect("desativa", code, 200, d)
expect("desativado não loga", local_login(uname, pw1), (401, 403))
call(IAM, "PATCH", f"users/{lid}", {"active": True, "display_name": "Local Teste"})
pw2 = f"Outra-senha-{RUN}#"
code, d = call(IAM, "POST", f"users/{lid}/password", {"password": pw2})
expect("redefine senha", code, (200, 204), d)
expect("senha antiga não vale", local_login(uname, pw1), 401)
expect("senha nova vale", local_login(uname, pw2), 200)
for _ in range(6):
    local_login(uname, "errada")
expect("conta bloqueada após 5 erros (lockout progressivo)", local_login(uname, pw2), 429)
code, d = call(IAM, "POST", f"users/{lid}/unlock")
expect("gestor desbloqueia", code, (200, 204), d)
time.sleep(61)  # o limitador de login por IP (10/min, penalizado pelos erros) abre nova janela
expect("login volta a funcionar (IP não bloqueado: limite próprio)", local_login(uname, pw2), 200)
code, d = call(IAM, "GET", f"users?q={uname}")
check("busca de usuários encontra", code == 200 and any(u["id"] == lid for u in d["data"]), code)
call(IAM, "PATCH", f"users/{lid}", {"active": False, "display_name": "Local Teste (desativado)"})

print("\n== Módulos: desligar, dependências e religar")
code, d = call(ADMIN, "PATCH", "admin/modules/signum", {"enabled": False})
expect("não desliga Signum com Trâmite ligado (dependência)", code, (409, 422), d)
code, d = call(ADMIN, "PATCH", "admin/modules/wiki", {"enabled": False})
expect("desliga Wiki", code, 200, d)
code, d = call(S, "GET", "wiki/tree")
check("rota da Wiki responde MODULE_DISABLED", code == 404 and "MODULE_DISABLED" in json.dumps(d), (code, d))
code, d = call(S, "GET", "system/modules")
check("Wiki desligada no estado dos módulos", any(m["key"] == "wiki" and not m["enabled"] for m in d["data"]))
code, d = call(ADMIN, "PATCH", "admin/modules/wiki", {"enabled": True})
expect("religa Wiki", code, 200, d)
expect("Wiki volta a responder", call(S, "GET", "wiki/tree")[0], 200)
code, _ = call(S, "PATCH", "admin/modules/wiki", {"enabled": False})
expect("servidor não mexe em módulos", code, 403)

print("\n== Branding (white-label)")
code, orig = call(ADMIN, "GET", "branding")
b = orig["data"]
code, d = call(ADMIN, "PUT", "admin/branding", {**b, "org_name": f"Município Fictício {RUN}", "support_email": "ouvidoria@nexus.test"})
expect("admin altera identidade visual", code, 200, d)
code, d = public("GET", "branding")
check("site público reflete a mudança", code == 200 and str(RUN) in json.dumps(d), d)
code, _ = call(S, "PUT", "admin/branding", {**b})
expect("servidor não altera branding", code, 403)
call(ADMIN, "PUT", "admin/branding", {**b, "org_name": "Prefeitura Municipal (teste)"})

print("\n== Auditoria")
code, d = call(AUD, "GET", "audit/logs?action=iam.entidade.saved")
expect("filtra por ação", code, 200)
if code == 200:
    check("encontra o CRUD feito acima", any(str(RUN) in json.dumps(x, ensure_ascii=False) for x in d["data"]), len(d["data"]))
    code, one = call(AUD, "GET", f"audit/logs/{d['data'][0]['id']}")
    expect("detalhe de um registro", code, 200, one)
code, d = call(AUD, "GET", "audit/verify")
check("cadeia SHA-256 íntegra", code == 200 and d["data"]["valid"] is True, d)
out = subprocess.run([NX, AUD, "GET", "audit/export?format=csv"], capture_output=True, text=True).stdout
check("exportação (LAI)", out.startswith("HTTP 200") and len(out) > 200, out[:200])

print("\n== Transparência (público) e monitoramento")
for p in ("transparencia/plataforma", "transparencia/modulos", "transparencia/datasets", "transparencia/auditoria/acoes"):
    code, _ = public("GET", p)
    expect(f"{p} (anônimo)", code, 200)
code, d = call(ADMIN, "GET", "monitoring/outbox-stats")
expect("estatísticas do outbox", code, 200, d)
if code == 200:
    print("   outbox:", d["data"])
code, _ = call(S, "GET", "monitoring/outbox-stats")
expect("servidor sem monitoramento", code, 403)

print("\n== LGPD: direitos do titular")
code, d = call(S, "GET", "lgpd/meus-dados")
check("meus dados (portabilidade)", code == 200 and S in json.dumps(d), code)
code, d = call(S, "POST", "lgpd/solicitar-exclusao", {"motivo": "Teste do fluxo de solicitação de exclusão"})
expect("solicita exclusão", code, (200, 201, 202, 409), d)
code, d = call(S, "GET", "lgpd/minhas-solicitacoes")
check("solicitação registrada", code == 200 and len(d["data"]) >= 1, d)

print("\n== Configuração do Keycloak e módulo Exemplo")
code, d = call(ADMIN, "GET", "admin/keycloak")
expect("admin lê a configuração do Keycloak", code, 200, d)
if code == 200:
    check("segredo nunca devolvido em claro", "client_secret\":\"" not in json.dumps(d) or "***" in json.dumps(d), d)
code, _ = call(S, "GET", "admin/keycloak")
expect("servidor não lê a configuração", code, 403)
# O módulo Exemplo nasce desativado (DefaultEnabled: false): liga para o
# teste e devolve ao estado em que estava.
code, d = call(ADMIN, "GET", "system/modules")
exemplo_ativo = code == 200 and any(m["key"] == "example" and m["enabled"] for m in d["data"])
if not exemplo_ativo:
    code, d = call(ADMIN, "PATCH", "admin/modules/example", {"enabled": True})
    expect("liga o módulo Exemplo", code, 200, d)
code, d = call(S, "POST", "examples", {"name": f"Item exemplo {RUN}"})
expect("cria item no módulo Exemplo", code, (200, 201, 403), d)
code, d = call(S, "GET", "examples")
expect("lista itens", code, (200, 403))
if not exemplo_ativo:
    code, d = call(ADMIN, "PATCH", "admin/modules/example", {"enabled": False})
    expect("desliga o módulo Exemplo de volta", code, 200, d)
done()
