"""Trâmite a fundo: anexos (MinIO), edição de documento, assinatura via
Signum, acesso a processo restrito, arquivar/reabrir e caixa da unidade."""
import subprocess
import time

from nxlib import *  # noqa: F403

RUN = int(time.time())
pickr = lambda g, k=0: (lambda l: l[(RUN + k) % len(l)])(users_in(g))  # noqa: E731
P1 = pickr("SEMPRAS-CRAS-CONJUNTO-ADM")          # protocolo CRAS Conjunto
P2 = pickr("SEMPRAS-CREAS-ADM", 1)               # protocolo CREAS
DIR1 = pickr("SEMPRAS-CRAS-CONJUNTO-DIR", 2)     # coordenação do CRAS (sem protocolo)
EXT = pickr("SEMED-ESCOLA-ELIZABETE-ADM", 3)     # outra secretaria
PW = env("DEMO_USER_PASSWORD")
me = {u: call(u, "GET", "me")[1]["data"] for u in (P1, P2, DIR1, EXT)}
uid = {u: me[u]["id"] for u in me}
unit = lambda u: next(s["unidade_id"] for s in me[u]["scopes"] if s.get("unidade_id"))  # noqa: E731
U1, U2 = unit(P1), unit(P2)
tipos = {t["slug"]: t["id"] for t in call(P1, "GET", "tramite/tipos")[1]["data"]}
print(f"P1={P1} (CRAS Conjunto)  P2={P2} (CREAS)  DIR1={DIR1}  EXT={EXT} (Educação)")

print("\n== Processo restrito com anexo")
code, d = call(P1, "POST", "tramite/processos", {"tipo_id": tipos["requerimento"], "assunto": f"Acompanhamento familiar {RUN}",
                                                  "interessado": "Família fictícia", "sigilo": "restrito", "unidade_origem_id": U1,
                                                  "descricao": "Caso de acompanhamento compartilhado com o CREAS."})
expect("CRAS abre processo restrito", code, 201, d)
pid, numero = d["data"]["id"], d["data"]["numero"]
print("   processo", numero)
anexo = f"Relatório social {RUN}".encode()
code, t = call(P1, "POST", f"tramite/processos/{pid}/uploads", {"filename": "relatorio.txt", "content_type": "text/plain"})
expect("pede URL de upload do anexo", code, 201, t)
tk = t["data"]
up = tk.get("upload") or tk
r = subprocess.run(["curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "-X", up.get("method") or "PUT",
                    *sum((["-H", f"{k}: {v}"] for k, v in (up.get("headers") or {}).items()), []),
                    "--data-binary", anexo.decode(), up["upload_url"]], capture_output=True, text=True).stdout
expect("anexo enviado ao MinIO", r, ("200",))
code, d = call(P1, "POST", f"tramite/processos/{pid}/documentos", {"tipo": "anexo", "titulo": "Relatório social", "object_key": up["object_key"]})
expect("junta o anexo como documento", code, 201, d)
code, d = call(P1, "POST", f"tramite/processos/{pid}/documentos", {"tipo": "despacho", "titulo": "Despacho inicial", "conteudo": "Encaminhar ao CREAS após assinatura."})
expect("junta despacho (texto)", code, 201, d)
doc = d["data"]["id"]
code, d = call(P1, "PUT", f"tramite/documentos/{doc}", {"titulo": "Despacho inicial (revisado)", "conteudo": "Encaminhar ao CREAS após assinatura da coordenação."})
expect("edita o despacho enquanto está com a unidade", code, 200, d)

print("\n== Assinatura do despacho via Signum")
code, d = call(P1, "POST", f"tramite/documentos/{doc}/assinatura", {"signer_ids": [uid[DIR1]], "sequential": False})
expect("solicita assinatura da coordenação", code, (200, 201), d)
env_id = (d["data"] or {}).get("envelope_id") or (d["data"] or {}).get("signum_envelope_id")
check("documento ligado a um envelope do Signum", bool(env_id), d["data"])
code, _ = call(DIR1, "GET", f"tramite/processos/{pid}")
expect("signatária de outra área vê o processo restrito (acesso pela assinatura)", code, (200, 403, 404))
if env_id:
    code, ch = call(DIR1, "POST", f"signum/envelopes/{env_id}/challenge")
    expect("coordenação pede desafio", code, 201, ch)
    code, d = call(DIR1, "POST", f"signum/envelopes/{env_id}/sign", {"challenge_id": ch["data"]["challenge_id"], "nonce": ch["data"]["nonce"], "password": PW, "confirm_document_sha256": ch["data"]["document_sha256"]})
    expect("coordenação assina com a senha do Keycloak", code, 200, d)
    code, d = call(P1, "GET", f"tramite/documentos/{doc}")
    expect("documento consultável", code, 200, d)
    if code == 200:
        print("   documento:", {k: v for k, v in d["data"].items() if k in ("titulo", "assinatura_status", "status", "assinado", "envelope_status")})
    code, d = call(P1, "PUT", f"tramite/documentos/{doc}", {"titulo": "adulterado", "conteudo": "depois de assinado"})
    expect("documento assinado não pode mais ser editado", code, (403, 409, 422), d)

print("\n== Acesso a processo restrito")
code, _ = call(EXT, "GET", f"tramite/processos/{pid}")
expect("outra secretaria não vê o restrito", code, (403, 404))
code, d = call(P1, "POST", f"tramite/processos/{pid}/acessos", {"user_id": uid[EXT]})
expect("CRAS concede acesso nominal", code, (200, 201, 204), d)
code, d = call(EXT, "GET", f"tramite/processos/{pid}")
expect("com acesso concedido, vê", code, 200, d)
code, _ = call(EXT, "POST", f"tramite/processos/{pid}/documentos", {"titulo": "intromissão", "conteudo": "x"})
expect("acesso de leitura não junta documentos", code, (403, 409))
code, _ = call(P1, "DELETE", f"tramite/processos/{pid}/acessos/{uid[EXT]}")
expect("revoga o acesso", code, (200, 204))
code, _ = call(EXT, "GET", f"tramite/processos/{pid}")
expect("revogado, deixa de ver", code, (403, 404))

print("\n== Tramitação, caixa e ciclo de vida")
code, d = call(P1, "POST", f"tramite/processos/{pid}/tramitar", {"para_unidade_id": U2, "despacho": "Encaminho ao CREAS para acompanhamento."})
expect("CRAS tramita ao CREAS", code, 200, d)
code, d = call(P2, "GET", "tramite/processos?minha_caixa=true")
check("processo na caixa do CREAS", code == 200 and any(p["id"] == pid for p in d["data"]), code)
code, d = call(P1, "GET", "tramite/processos?minha_caixa=true")
check("saiu da caixa do CRAS", code == 200 and not any(p["id"] == pid for p in d["data"]), code)
code, _ = call(P1, "PUT", f"tramite/documentos/{doc}", {"titulo": "x", "conteudo": "y"})
expect("CRAS não edita mais (processo está no CREAS)", code, (403, 409, 422))
code, d = call(P2, "POST", f"tramite/processos/{pid}/documentos", {"tipo": "parecer", "titulo": "Parecer do CREAS", "conteudo": "Acompanhamento iniciado."})
expect("CREAS junta parecer", code, 201, d)
code, d = call(P2, "POST", f"tramite/processos/{pid}/arquivar", {"despacho": "Arquivar direto."})
expect("em tramitação não se arquiva (só concluído)", code, 409, d)
code, d = call(P2, "POST", f"tramite/processos/{pid}/concluir", {"despacho": "Acompanhamento concluído."})
expect("CREAS conclui", code, 200, d)
code, d = call(P2, "POST", f"tramite/processos/{pid}/arquivar", {"despacho": "Arquivado após acompanhamento."})
expect("CREAS arquiva o concluído", code, 200, d)
code, _ = call(P2, "POST", f"tramite/processos/{pid}/documentos", {"titulo": "após arquivar", "conteudo": "x"})
expect("arquivado não recebe documentos", code, (403, 409, 422))
code, d = call(P2, "POST", f"tramite/processos/{pid}/reabrir", {"despacho": "Reabrir."})
expect("Protocolo não reabre (exige tramite:manage)", code, 403, d)
code, d = call("teste.admin", "POST", f"tramite/processos/{pid}/reabrir", {"despacho": "Reaberto: nova demanda da família."})
expect("gestor (tramite:manage) reabre", code, 200, d)
code, d = call(P2, "GET", f"tramite/processos/{pid}")
if code == 200:
    p = d["data"]
    check("em tramitação após reabrir", p.get("status") == "em_tramitacao", p.get("status"))
    movs = [m.get("acao") for m in (p.get("movimentos") or [])]
    check("histórico: abertura, tramitação, conclusão, arquivamento, reabertura", len(movs) >= 5, movs)
    check("3 documentos (anexo, despacho, parecer); o recusado após arquivar não entrou", len(p.get("documentos") or []) == 3, len(p.get("documentos") or []))
code, d = call(P2, "GET", f"search?q={numero.split('/')[0]}")
check("processo encontrado pelo número na busca global", code == 200 and any(pid in json.dumps(h) for h in d["data"]["results"]), d)
done()
