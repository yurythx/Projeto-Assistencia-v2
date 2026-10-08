"""Povoa TODOS os apps com dados fictícios realistas, criados pelos
próprios usuários fictícios pela API (mesmo caminho do navegador): cada
registro nasce com autoria, auditoria, eventos e permissões reais.

    python3 popular.py           # só roda uma vez (marca em .demo-popular)
    python3 popular.py --force   # roda de novo (cria duplicatas)
"""
import hashlib
import subprocess
import sys
import time
import urllib.request
from datetime import datetime, timedelta, timezone

from nxlib import *  # noqa: F403

MARK = os.path.join(ROOT, ".demo-popular")
if os.path.exists(MARK) and "--force" not in sys.argv:
    print(f"Já povoado em {open(MARK).read().strip()} — use --force para repetir.")
    sys.exit(0)

ADMIN, EDITOR = "teste.admin", "teste.conteudo"
PW = env("DEMO_USER_PASSWORD")
FE = env("FRONTEND_URL")
first = lambda g, i=0: users_in(g)[i]  # noqa: E731
_me = {}


def me(u):
    if u not in _me:
        _me[u] = call(u, "GET", "me")[1]["data"]
    return _me[u]


def unit_of(u):
    return next(s["unidade_id"] for s in me(u)["scopes"] if s.get("unidade_id"))


def ok(label, code, d, want=(200, 201)):
    if code not in want:
        print(f"  ! {label}: HTTP {code} {str(d)[:200]}")
        FAILS.append(label)
        return None
    return d["data"] if isinstance(d, dict) else d


unidades = {u["nome"]: u["id"] for u in call(ADMIN, "GET", "iam/unidades")[1]["data"]}
now = datetime.now(timezone.utc).replace(minute=0, second=0, microsecond=0)
iso = lambda dt: dt.strftime("%Y-%m-%dT%H:%M:%SZ")  # noqa: E731

print("== Catálogo de serviços (Carta de Serviços)")
SERVICOS = [
    ("Agendamento de consultas no PSF", "Saúde", "PSF Sagrada Família", "Marque consultas de clínica geral, pré-natal e puericultura.",
     ["Cartão SUS", "Documento com foto", "Comprovante de residência"], ["Compareça ao PSF do seu bairro", "Agende na recepção", "Aguarde a confirmação"], "Até 7 dias", "presencial"),
    ("Atendimento de urgência e emergência", "Saúde", "UPA 24h", "Pronto atendimento 24 horas com classificação de risco.",
     ["Documento com foto", "Cartão SUS (se tiver)"], ["Classificação de risco", "Atendimento médico", "Encaminhamento, se necessário"], "Imediato, conforme risco", "presencial"),
    ("Vacinação de rotina e campanhas", "Saúde", "PSF Conjunto", "Aplicação das vacinas do calendário nacional e das campanhas sazonais.",
     ["Cartão de vacinação", "Documento com foto"], ["Leve o cartão de vacinação", "Procure a sala de vacina"], "No ato", "presencial"),
    ("Matrícula na rede municipal de ensino", "Educação", "Escola Maria Elza", "Matrícula e rematrícula na educação infantil e no ensino fundamental.",
     ["Certidão de nascimento", "CPF do aluno e do responsável", "Comprovante de residência", "Cartão de vacinação"], ["Consulte o período de matrícula", "Compareça à escola", "Entregue os documentos"], "No ato, conforme vagas", "presencial"),
    ("Transporte escolar", "Educação", "Escola Marechal Dutra", "Transporte gratuito para alunos da zona rural e de áreas distantes.",
     ["Comprovante de matrícula", "Comprovante de residência"], ["Solicite na secretaria da escola", "Aguarde a análise de rota"], "Até 15 dias", "presencial"),
    ("Declaração de escolaridade", "Educação", "Escola Silvestre", "Emissão de declaração de matrícula e frequência escolar.",
     ["Documento do responsável"], ["Solicite na secretaria da escola"], "Até 2 dias úteis", "presencial"),
    ("Cadastro Único (CadÚnico)", "Assistência Social", "CRAS Conjunto", "Inscrição e atualização no Cadastro Único para programas sociais.",
     ["CPF ou título de eleitor de todos da família", "Comprovante de residência", "Comprovante de renda"], ["Agende no CRAS", "Entrevista com o cadastrador"], "No ato da entrevista", "presencial"),
    ("Benefícios eventuais", "Assistência Social", "CRAS Ana Carla", "Auxílio natalidade, funeral e situações de vulnerabilidade temporária.",
     ["Documento com foto", "Comprovante de residência"], ["Atendimento com assistente social", "Análise do caso"], "Até 10 dias", "presencial"),
    ("Acolhimento da população em situação de rua", "Assistência Social", "Centro POP", "Higiene, alimentação, documentação e encaminhamentos.",
     ["Não exige documentos"], ["Procure o Centro POP de segunda a sexta"], "Imediato", "presencial"),
    ("Protocolo geral", "Administração", "Sede Administrativa da Prefeitura", "Abertura de requerimentos e processos administrativos.",
     ["Documento com foto", "Requerimento preenchido"], ["Preencha o requerimento", "Protocole no atendimento", "Acompanhe pelo número"], "Até 30 dias", "online"),
    ("Ouvidoria municipal", "Administração", "Sede Administrativa da Prefeitura", "Reclamações, sugestões, elogios e denúncias.",
     ["Nenhum"], ["Use o formulário de contato do portal"], "Resposta em até 30 dias (Lei 13.460/2017)", "online"),
]
for title, cat, unidade, summary, reqs, steps, sla, canal in SERVICOS:
    ch = [{"type": canal, "label": unidade if canal == "presencial" else "Portal do cidadão", "value": "Horário de funcionamento da unidade" if canal == "presencial" else FE + "/contato"}]
    d = ok(f"serviço {title}", *call(EDITOR, "POST", "catalog/admin/services", {"title": title, "summary": summary, "description": summary, "category": cat,
                                                                              "requirements": reqs, "steps": steps, "sla": sla, "cost": "Gratuito",
                                                                              "channels": ch, "responsible_unidade_id": unidades.get(unidade)}))
    if d:
        ok(f"publica {title}", *call(EDITOR, "POST", f"catalog/admin/services/{d['id']}/publish"))
print(f"   {len(SERVICOS)} serviços publicados")

print("== Blog (notícias e comunicados)")
POSTS = [
    ("Campanha de vacinação contra a gripe começa na segunda", "noticia", "Todas as unidades de saúde aplicarão a vacina a partir das 7h."),
    ("UPA 24h amplia equipe no plantão noturno", "noticia", "Dois novos médicos passam a atender no período noturno."),
    ("Período de matrículas da rede municipal", "comunicado", "As matrículas para o próximo ano letivo ficam abertas por 15 dias."),
    ("Mutirão do CadÚnico nos CRAS", "noticia", "Atendimento estendido para inscrição e atualização cadastral."),
    ("Recesso administrativo", "comunicado", "Expediente reduzido na sede administrativa na próxima sexta-feira."),
    ("Nova plataforma Nexus entra em operação", "comunicado", "Processos, assinaturas e comunicação interna em um só lugar."),
]
for title, kind, body in POSTS:
    d = ok(f"post {title}", *call(EDITOR, "POST", "blog/posts", {"title": title, "summary": body, "body": body + "\n\nMais informações na unidade mais próxima.", "kind": kind}))
    if d:
        ok(f"publica {title}", *call(EDITOR, "POST", f"blog/posts/{d['id']}/publish"))
d = ok("rascunho", *call(EDITOR, "POST", "blog/posts", {"title": "Rascunho: balanço do trimestre", "summary": "Em elaboração", "body": "..."}))
print(f"   {len(POSTS)} publicados + 1 rascunho")

print("== Wiki (manuais por área)")
WIKI = {
    "Manual do servidor": ["Primeiro acesso ao Nexus", "Como abrir um processo", "Como assinar documentos", "Compartilhando arquivos"],
    "Saúde: rotinas das unidades": ["Classificação de risco na UPA", "Sala de vacina: conservação", "Agenda do PSF"],
    "Educação: secretaria escolar": ["Matrícula e rematrícula", "Transferência de alunos", "Transporte escolar"],
    "Assistência social: atendimento": ["Acolhida no CRAS", "Encaminhamento ao CREAS", "Atendimento no Centro POP"],
}
autores = [first("PREF-SEDE-TI", 1), first("SEMSA-UPA-DIR"), first("SEMED-ESCOLA-MARIA-ELZA-DIR"), first("SEMPRAS-SEDE-DIR")]
for (root, kids), autor in zip(WIKI.items(), autores):
    r = ok(f"wiki {root}", *call(autor, "POST", "wiki/pages", {"title": root, "body": f"# {root}\n\nÍndice das rotinas desta área.", "summary": "criação"}))
    for i, k in enumerate(kids):
        if r:
            ok(f"wiki {k}", *call(autor, "POST", "wiki/pages", {"title": k, "parent_id": r["id"], "position": i,
                                                                 "body": f"# {k}\n\n1. Passo inicial\n2. Conferência\n3. Registro no sistema", "summary": "criação"}))
print(f"   {sum(1 + len(v) for v in WIKI.values())} páginas")

print("== Agenda (salas e eventos)")
salas = {}
for nome, loc, cap in (("Auditório da Prefeitura", "Sede Administrativa", 120), ("Sala de reuniões da SEMSA", "Secretaria de Saúde", 20),
                       ("Sala de reuniões da SEMED", "Secretaria de Educação", 20), ("Sala de reuniões da SEMPRAS", "Sede da SEMPRAS", 15)):
    d = ok(f"sala {nome}", *call(ADMIN, "POST", "calendar/rooms", {"name": nome, "location": loc, "capacity": cap, "resources": ["projetor", "videoconferência"]}))
    if d:
        salas[nome] = d["id"]
EVENTOS = [
    (ADMIN, "Dia D de vacinação", 3, 8, 17, None, "public", "Todas as unidades de saúde"),
    (ADMIN, "Audiência pública do orçamento", 10, 19, 21, "Auditório da Prefeitura", "public", ""),
    (ADMIN, "Feira de matrículas", 6, 9, 16, None, "public", "Praça central"),
    (first("SEMSA-UPA-DIR"), "Reunião de coordenadores da saúde", 2, 14, 16, "Sala de reuniões da SEMSA", "internal", ""),
    (first("SEMED-ESCOLA-SILVESTRE-DIR"), "Conselho de diretores escolares", 4, 9, 11, "Sala de reuniões da SEMED", "internal", ""),
    (first("SEMPRAS-SEDE-DIR"), "Estudo de caso CRAS/CREAS", 5, 14, 17, "Sala de reuniões da SEMPRAS", "internal", ""),
    (first("PREF-SEDE-TI"), "Treinamento Nexus: tramitação", 7, 9, 12, "Auditório da Prefeitura", "internal", ""),
    (first("PREF-SEDE-RH"), "Integração de novos servidores", 8, 8, 12, "Auditório da Prefeitura", "internal", ""),
    (first("PREF-SEDE-CONT"), "Fechamento contábil", 12, 9, 12, None, "private", ""),
]
for autor, t, dd, h1, h2, sala, vis, loc in EVENTOS:
    day = (now + timedelta(days=dd)).replace(hour=0)
    body = {"title": t, "starts_at": iso(day + timedelta(hours=h1 + 4)), "ends_at": iso(day + timedelta(hours=h2 + 4)), "visibility": vis,
            "location": loc, "description": f"{t} — evento fictício de teste."}
    if sala:
        body["room_id"] = salas.get(sala)
    ok(f"evento {t}", *call(autor, "POST", "calendar/events", body))
print(f"   {len(salas)} salas, {len(EVENTOS)} eventos")

print("== Mercúrio (salas de equipe e conversas)")
for grupo, nome in (("SEMSA-UPA-ADM", "Administração da UPA"), ("SEMED-ESCOLA-MARIA-ELZA-DIR", "Direção — Escola Maria Elza"),
                    ("SEMPRAS-CRAS-CONJUNTO-ATD", "Atendimento — CRAS Conjunto"), ("PREF-SEDE-TI", "TI da Prefeitura")):
    d = ok(f"sala {nome}", *call(ADMIN, "POST", "mercurio/rooms", {"kind": "department", "name": nome, "ad_group": grupo, "description": "Canal da equipe"}))
    if d:
        membros = users_in(grupo)
        for i, msg in enumerate(["Bom dia, equipe!", "Escala da semana publicada na pasta da unidade.", "Alguém pode cobrir o plantão de sexta?", "Eu cubro."]):
            ok(f"msg {nome}", *call(membros[i % len(membros)], "POST", f"mercurio/rooms/{d['id']}/messages", {"body": msg}))
geral = next(r["id"] for r in call(ADMIN, "GET", "mercurio/rooms")[1]["data"] if r["kind"] == "global")
for u, msg in ((first("PREF-SEDE-GOV"), "Sejam bem-vindos ao Nexus! Dúvidas, falem com a TI."), (first("PREF-SEDE-TI"), "Manual do servidor já está na Wiki."),
               (first("SEMSA-UPA-DIR"), "UPA com movimento alto hoje, priorizem os chamados urgentes.")):
    ok("msg geral", *call(u, "POST", f"mercurio/rooms/{geral}/messages", {"body": msg}))
a, b = first("SEMSA-PSF-SAGRADA-FAMILIA-DIR"), first("SEMSA-UPA-DIR")
d = ok("conversa direta", *call(a, "POST", "mercurio/direct", {"user_id": me(b)["id"]}))
if d:
    for u, msg in ((a, "Pode receber um paciente transferido do PSF hoje?"), (b, "Pode sim, avise a recepção quando sair."), (a, "Obrigado!")):
        ok("msg direta", *call(u, "POST", f"mercurio/rooms/{d['id']}/messages", {"body": msg}))

print("== Trâmite (processos entre unidades)")
tipos = {t["slug"]: t["id"] for t in call(ADMIN, "GET", "tramite/tipos")[1]["data"]}
FLUXOS = [  # (origem ADM, assunto, tipo, sigilo, [destinos ADM], fim)
    ("SEMSA-UPA-ADM", "Solicitação de reposição de medicamentos", "requerimento", "publico", ["PREF-SEDE-COMP"], "tramitando"),
    ("SEMSA-PSF-CONJUNTO-ADM", "Manutenção do ar-condicionado da sala de vacina", "memorando", "publico", ["PREF-SEDE-ADM", "SEMSA-PSF-CONJUNTO-ADM"], "concluir"),
    ("SEMED-ESCOLA-SILVESTRE-ADM", "Pedido de transporte escolar — linha rural", "requerimento", "publico", ["SEMED-ESCOLA-MARECHAL-DUTRA-ADM"], "tramitando"),
    ("SEMED-ESCOLA-ELIZABETE-ADM", "Reforma da quadra poliesportiva", "contratacao", "publico", ["PREF-SEDE-COMP", "PREF-SEDE-CONT"], "tramitando"),
    ("SEMPRAS-CRAS-ANA-CARLA-ADM", "Encaminhamento de família para acompanhamento especializado", "outros", "restrito", ["SEMPRAS-CREAS-ADM"], "tramitando"),
    ("SEMPRAS-CENTRO-POP-ADM", "Aquisição de kits de higiene", "contratacao", "publico", ["PREF-SEDE-COMP"], "arquivar"),
    ("PREF-SEDE-RH", "Convocação para curso de capacitação", "memorando", "publico", [], "concluir"),
    ("PREF-SEDE-ADM", "Ofício ao Ministério da Saúde sobre repasses", "oficio", "publico", ["SEMSA-UPA-ADM"], "tramitando"),
]
nproc = 0
for g_origem, assunto, tipo, sigilo, destinos, fim in FLUXOS:
    autor = first(g_origem)
    if not any(s["perfil"] == "protocolo" for s in me(autor)["scopes"]):
        autor = first(g_origem.rsplit("-", 1)[0] + "-ADM")
    p = ok(f"processo {assunto}", *call(autor, "POST", "tramite/processos", {"tipo_id": tipos[tipo], "assunto": assunto, "interessado": "Unidade requisitante",
                                                                            "sigilo": sigilo, "unidade_origem_id": unit_of(autor), "descricao": assunto + "."}))
    if not p:
        continue
    nproc += 1
    ok("despacho", *call(autor, "POST", f"tramite/processos/{p['id']}/documentos", {"tipo": "despacho", "titulo": "Despacho inicial", "conteudo": f"Solicito providências: {assunto.lower()}."}))
    atual = autor
    for g in destinos:
        dest = first(g)
        if not any(s["perfil"] == "protocolo" for s in me(dest)["scopes"]):
            dest = first(g.rsplit("-", 1)[0] + "-ADM")
        if unit_of(dest) == unit_of(atual):  # departamentos da mesma unidade (ex.: sede da Prefeitura)
            continue
        ok("tramita", *call(atual, "POST", f"tramite/processos/{p['id']}/tramitar", {"para_unidade_id": unit_of(dest), "despacho": "Encaminho para análise e providências."}))
        ok("parecer", *call(dest, "POST", f"tramite/processos/{p['id']}/documentos", {"tipo": "parecer", "titulo": "Parecer", "conteudo": "Analisado. Segue para os próximos passos."}))
        atual = dest
    if fim == "concluir":
        ok("conclui", *call(atual, "POST", f"tramite/processos/{p['id']}/concluir", {"despacho": "Demanda atendida."}))
    elif fim == "arquivar":  # só se arquiva o que foi concluído
        ok("conclui", *call(atual, "POST", f"tramite/processos/{p['id']}/concluir", {"despacho": "Compra consolidada em outro processo."}))
        ok("arquiva", *call(atual, "POST", f"tramite/processos/{p['id']}/arquivar", {"despacho": "Arquivado: compra consolidada em outro processo."}))
print(f"   {nproc} processos")

print("== Signum (documentos assinados)")
for dono_g, title, signers_g, assinar in (("PREF-SEDE-GOV", "Portaria de nomeação de comissão", ["PREF-SEDE-ADM", "PREF-SEDE-RH"], True),
                                          ("SEMSA-UPA-DIR", "Escala de plantão médico", ["SEMSA-UPA-RH"], True),
                                          ("SEMED-ESCOLA-MARIA-ELZA-DIR", "Ata do conselho escolar", ["SEMED-ESCOLA-MARIA-ELZA-ADM", "SEMED-ESCOLA-MARIA-ELZA-ATD"], False)):
    sha = hashlib.sha256(title.encode()).hexdigest()
    signers = [first(g) for g in signers_g]
    e = ok(f"envelope {title}", *call(first(dono_g), "POST", "signum/envelopes", {"title": title, "description": "Documento fictício", "document_sha256": sha,
                                                                                   "signer_ids": [me(s)["id"] for s in signers], "sequential": True}))
    if e and assinar:
        for s in signers:
            c, ch = call(s, "POST", f"signum/envelopes/{e['id']}/challenge")
            if ok("desafio", c, ch):
                ok(f"assina {title}", *call(s, "POST", f"signum/envelopes/{e['id']}/sign", {"challenge_id": ch["data"]["challenge_id"], "nonce": ch["data"]["nonce"],
                                                                                         "password": PW, "confirm_document_sha256": sha}))

print("== Arquivos (pasta de cada unidade, compartilhada com a unidade)")
nf = 0
for uni_nome, uni_id in sorted(unidades.items()):
    dono = next((u["usuario"] for u in USERS if u["unidade"] == uni_nome and "-ADM" in u["grupos"]), None) or \
        next((u["usuario"] for u in USERS if u["unidade"] == uni_nome), None)
    if not dono:
        continue
    f = ok(f"pasta {uni_nome}", *call(dono, "POST", "files/folders", {"name": f"Documentos — {uni_nome}"}))
    if not f:
        continue
    ok("acl", *call(dono, "PUT", f"files/folders/{f['id']}/acl", {"entries": [{"subject_type": "unidade", "subject": uni_id, "can_write": True}]}))
    conteudo = f"Escala e contatos da unidade {uni_nome} (fictício).".encode()
    c, t = call(dono, "POST", "files/uploads", {"folder_id": f["id"], "filename": "escala-e-contatos.txt", "content_type": "text/plain", "size": len(conteudo)})
    if ok("upload", c, t):
        r = subprocess.run(["curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "-X", "PUT", "-H", "Content-Type: text/plain",
                            "--data-binary", conteudo.decode(), t["data"]["upload"]["upload_url"]], capture_output=True, text=True).stdout
        if r == "200":
            ok("confirma", *call(dono, "POST", f"files/objects/{t['data']['file']['id']}/confirm"))
            nf += 1
print(f"   {nf} pastas com arquivo")

print("== Contato (mensagens de cidadãos pelo site)")
MSGS = [("Maria Cidadã", "duvida", "Horário da sala de vacina", "Qual o horário de funcionamento da sala de vacina do PSF Conjunto?"),
        ("João Munícipe", "reclamacao", "Demora no atendimento", "Esperei muito tempo na recepção da UPA ontem à noite."),
        ("Ana Moradora", "elogio", "Atendimento no CRAS", "Fui muito bem atendida no CRAS Ana Carla, parabéns à equipe."),
        ("Pedro Pai de Aluno", "sugestao", "Transporte escolar", "Sugiro um ponto de ônibus escolar próximo ao bairro novo."),
        ("Carla Contribuinte", "outro", "Certidão negativa", "Como solicito a certidão negativa de débitos municipais?")]
for i, (nome, cat, assunto, msg) in enumerate(MSGS):
    req = urllib.request.Request(f"{FE}/api/public/v1/contact/messages", method="POST", headers={"Content-Type": "application/json"},
                                 data=json.dumps({"name": nome, "email": f"cidadao{i}@exemplo.test", "subject": assunto, "category": cat,
                                                  "message": msg, "consent": True, "website": ""}).encode())
    try:
        urllib.request.urlopen(req, timeout=15, context=SSL_CTX)
    except urllib.error.HTTPError as e:
        if e.code == 429:  # limite do formulário por IP (5/h): proteção, não erro
            print("   formulário de contato no limite por IP — o restante fica para depois")
            break
        print("  ! contato:", e.code, e.read()[:200])
        FAILS.append("contato")
caixa = call(EDITOR, "GET", "contact/messages")[1]["data"]
for m in caixa[:2]:
    ok("triagem", *call(EDITOR, "PATCH", f"contact/messages/{m['id']}", {"status": "in_progress", "notes": "Encaminhado à unidade responsável."}))
print(f"   {len(MSGS)} mensagens")

if FAILS:
    print(f"\n{len(FAILS)} falha(s): {sorted(set(FAILS))}")
    sys.exit(1)
open(MARK, "w").write(time.strftime("%Y-%m-%d %H:%M:%S"))
print("\nPovoamento concluído.")
