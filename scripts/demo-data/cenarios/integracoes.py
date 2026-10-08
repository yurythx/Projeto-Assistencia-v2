"""Integrações entre módulos: webhooks de saída (outbox -> RabbitMQ ->
worker -> HTTP), busca global sobre conteúdo de vários módulos e a trilha
de auditoria de cada tipo de ação."""
import time

from nxlib import *  # noqa: F403

RUN = int(time.time())
ADMIN, EDITOR, AUD = "teste.admin", "teste.conteudo", "teste.auditor"
S = (lambda l: l[RUN % len(l)])(users_in("SEMSA-PSF-CONJUNTO-ATD"))
print(f"servidor={S}")


def wait(fn, timeout=60, step=2):
    end = time.time() + timeout
    while time.time() < end:
        v = fn()
        if v:
            return v
        time.sleep(step)
    return None


print("\n== Webhooks de saída (Egress)")
code, d = call(ADMIN, "POST", "egress/targets", {"name": f"Receptor OK {RUN}", "kind": "webhook", "url": "https://httpbin.org/post",
                                                  "secret": f"segredo-hmac-{RUN}", "event_patterns": ["catalog.service.*"]})
expect("cadastra destino (com segredo HMAC)", code, 201, d)
ok_t = d["data"]["id"]
check("segredo não volta na resposta", "segredo-hmac" not in json.dumps(d) and d["data"].get("has_secret") is True, d["data"])
code, d = call(ADMIN, "POST", "egress/targets", {"name": f"Receptor com erro {RUN}", "kind": "webhook", "url": "https://httpbin.org/status/500",
                                                  "event_patterns": ["catalog.service.*"]})
expect("cadastra destino que responde 500", code, 201, d)
bad_t = d["data"]["id"]
code, d = call(ADMIN, "POST", f"egress/targets/{ok_t}/test")
expect("teste de conexão do destino", code, (200, 201, 202), d)

code, d = call(EDITOR, "POST", "catalog/admin/services", {"title": f"Serviço com webhook {RUN}", "summary": "Gera eventos", "category": "Testes",
                                                           "channels": [{"type": "online", "label": "Portal", "value": "https://exemplo.test"}]})
sid = d["data"]["id"]
call(EDITOR, "POST", f"catalog/admin/services/{sid}/publish")


def deliveries(target):
    c, x = call(ADMIN, "GET", f"egress/deliveries?target_id={target}")
    return [v for v in (x.get("data") or []) if v.get("target_id") == target] if c == 200 else []


ok = wait(lambda: [v for v in deliveries(ok_t) if v["status"] in ("delivered", "succeeded", "sent")], 90)
check("evento do catálogo entregue no receptor (outbox -> RabbitMQ -> worker -> HTTP)", bool(ok), deliveries(ok_t))
if ok:
    print("   entregas OK:", sorted({v["event_type"] for v in ok}))
bad = wait(lambda: [v for v in deliveries(bad_t) if v["attempts"] >= 2 or v["status"] in ("failed", "dead")], 120, 5)
check("destino com erro: nova tentativa com backoff", bool(bad), deliveries(bad_t))
if bad:
    b = bad[0]
    print("   entrega com erro:", {k: b.get(k) for k in ("status", "attempts", "next_attempt_at", "last_error")})
    code, d = call(ADMIN, "POST", f"egress/deliveries/{b['id']}/redeliver")
    expect("reenvio manual", code, (200, 202, 204), d)
code, d = call(ADMIN, "PUT", f"egress/targets/{bad_t}", {"name": f"Receptor com erro {RUN}", "kind": "webhook", "url": "https://httpbin.org/status/500",
                                                          "event_patterns": ["catalog.service.*"], "active": False})
expect("desativa destino com erro", code, 200, d)
for t in (ok_t, bad_t):
    call(ADMIN, "DELETE", f"egress/targets/{t}")

print("\n== Busca global sobre vários módulos")
code, d = call(EDITOR, "POST", "blog/posts", {"title": f"Notícia pesquisável {RUN}", "summary": "s", "body": f"conteúdo {RUN}", "kind": "noticia"})
post_id = d["data"]["id"]
call(EDITOR, "POST", f"blog/posts/{post_id}/publish")
code, d = call(S, "POST", "wiki/pages", {"title": f"Procedimento pesquisável {RUN}", "body": f"passo a passo {RUN}", "summary": "criação"})
wiki_id = d["data"]["id"]
mods = lambda q: (lambda r: sorted({h["module"] for h in r[1]["data"]["results"]}) if r[0] == 200 else r)(call(S, "GET", f"search?q={q}"))  # noqa: E731
kinds = wait(lambda: (lambda k: k if isinstance(k, list) and len(k) >= 3 else None)(mods(RUN)), 30) or mods(RUN)
print("   resultados por módulo:", kinds)
check("busca encontra blog, wiki e catálogo criados agora", isinstance(kinds, list) and {"blog", "wiki", "catalog"} <= set(kinds), kinds)
kinds = mods("pesquis")
check("prefixo 'pesquis' acha 'pesquisável' (blog e wiki)", isinstance(kinds, list) and {"blog", "wiki"} <= set(kinds), kinds)
code, d = call(S, "GET", "search?q=a")
expect("termo curto recusado", code, 422)

print("\n== Trilha de auditoria por tipo de ação")
# A trilha guarda o id do recurso (e, nas criações, o conteúdo): procura por ele.
for action, rid in (("catalog.service.published", sid), ("blog.post.published", post_id), ("wiki.page.created", wiki_id), ("egress.target.saved", ok_t)):
    code, d = call(AUD, "GET", f"audit/logs?action={action}")
    check(f"auditado: {action}", code == 200 and any(rid in json.dumps(x) for x in d["data"]), code)
code, d = call(AUD, "GET", "audit/verify")
check("cadeia íntegra depois de tudo", code == 200 and d["data"]["valid"] is True, d)
done()
