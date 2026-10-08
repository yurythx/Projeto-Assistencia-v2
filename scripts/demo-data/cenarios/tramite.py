from nxlib import *  # noqa: F403

upa_adm, upa_atd = users_in("SEMSA-UPA-ADM")[1], users_in("SEMSA-UPA-ATD")[1]
psf_adm = users_in("SEMSA-PSF-SAGRADA-FAMILIA-ADM")[1]
esc_adm = users_in("SEMED-ESCOLA-SILVESTRE-ADM")[1]
print(f"protocolo UPA={upa_adm}  atendimento UPA={upa_atd}  protocolo PSF={psf_adm}  protocolo escola={esc_adm}")

_, me = call(upa_adm, "GET", "me")
upa_id = next(s["unidade_id"] for s in me["data"]["scopes"] if s["perfil"] == "protocolo")
_, me = call(psf_adm, "GET", "me")
psf_id = next(s["unidade_id"] for s in me["data"]["scopes"] if s["perfil"] == "protocolo")
_, tipos = call(upa_adm, "GET", "tramite/tipos")
memo = next(t["id"] for t in tipos["data"] if t["slug"] == "memorando")

print("\n== criação")
code, d = call(upa_atd, "POST", "tramite/processos", {"tipo_id": memo, "assunto": "Sem perfil protocolo", "sigilo": "publico", "unidade_origem_id": upa_id})
expect("servidor sem Protocolo não abre processo", code, (403,))
code, d = call(upa_adm, "POST", "tramite/processos", {"tipo_id": memo, "assunto": "Protocolo em outra unidade", "sigilo": "publico", "unidade_origem_id": psf_id})
expect("Protocolo da UPA não abre processo em nome do PSF", code, (403, 422))
code, d = call(upa_adm, "POST", "tramite/processos", {"tipo_id": memo, "assunto": "Solicitação de insumos para a UPA", "interessado": "UPA 24h", "descricao": "Reposição de material.", "sigilo": "publico", "unidade_origem_id": upa_id})
expect("Protocolo da UPA abre memorando", code, (200, 201), d)
pid = d["data"]["id"] if code in (200, 201) else None
if pid: print("  processo:", d["data"].get("numero"), pid)

code, d = call(upa_adm, "POST", "tramite/processos", {"tipo_id": memo, "assunto": "Sindicância interna", "sigilo": "restrito", "unidade_origem_id": upa_id})
rid = d["data"]["id"] if code in (200, 201) else None
expect("Protocolo da UPA abre processo restrito", code, (200, 201), d)

print("\n== visibilidade")
if rid:
    code, _ = call(esc_adm, "GET", f"tramite/processos/{rid}")
    expect("restrito da UPA invisível para a escola", code, (403, 404))
    code, d = call(esc_adm, "GET", "tramite/processos")
    ids = [p["id"] for p in (d.get("data") or [])] if isinstance(d, dict) else []
    check("restrito da UPA fora da listagem da escola", rid not in ids)

print("\n== tramitação UPA -> PSF -> UPA")
if pid:
    code, d = call(psf_adm, "POST", f"tramite/processos/{pid}/tramitar", {"para_unidade_id": upa_id, "despacho": "Tentativa indevida"})
    expect("PSF não tramita processo que está na UPA", code, (403, 409, 422))
    code, d = call(upa_adm, "POST", f"tramite/processos/{pid}/tramitar", {"para_unidade_id": psf_id, "despacho": "Encaminho para ciência do PSF."})
    expect("UPA tramita para o PSF", code, (200, 201, 204), d)
    code, d = call(psf_adm, "GET", f"tramite/processos/{pid}")
    expect("PSF vê o processo recebido", code, 200, d)
    if code == 200:
        check("unidade atual = PSF", d["data"].get("unidade_atual_id") == psf_id, d["data"].get("unidade_atual_id"))
    code, d = call(psf_adm, "POST", f"tramite/processos/{pid}/documentos", {"tipo": "despacho", "titulo": "Despacho do PSF", "conteudo": "Ciente. Devolvo."})
    expect("PSF junta documento", code, (200, 201), d)
    code, d = call(psf_adm, "POST", f"tramite/processos/{pid}/tramitar", {"para_unidade_id": upa_id, "despacho": "Devolvo à origem."})
    expect("PSF devolve à UPA", code, (200, 201, 204), d)
    code, d = call(upa_adm, "POST", f"tramite/processos/{pid}/concluir", {"despacho": "Atendido."})
    expect("UPA conclui", code, (200, 201, 204), d)
    code, d = call(upa_adm, "GET", f"tramite/processos/{pid}")
    if code == 200:
        check("status concluído", d["data"].get("status") in ("concluido", "concluído"), d["data"].get("status"))
        movs = d["data"].get("movimentos") or []
        check("histórico de movimentos registrado (>=3)", len(movs) >= 3, len(movs))
done()
