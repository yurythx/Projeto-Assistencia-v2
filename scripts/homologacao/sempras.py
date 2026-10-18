#!/usr/bin/env python3
"""Kit de homologação da SEMPRAS (ADR 031): numa stack de TESTE, cria um
usuário local por perfil (recepção, técnico, psicologia) para uma unidade de
cada tipo — mais a superintendência — e povoa munícipes e atendimentos
fictícios por tipo, para a equipe testar a própria rotina com login real.

    python3 scripts/homologacao/sempras.py \\
        --api http://localhost:8002/api/v1 --admin admin --senha-admin '<senha>' \\
        --senha 'Homolog-2026!'

Pré-requisitos na stack: a estrutura real (make estrutura-aplicar), as
migrations, os tipos das unidades (deploy/sempras/tipos-unidades.sql) e o
módulo SEMPRAS ligado. Idempotente quanto aos usuários (reaproveita quem já
existe); os dados fictícios são criados de novo a cada execução.

Recusa-se a rodar contra a produção (--producao-host): homologação não cria
usuário nem munícipe fictício no banco de verdade.
"""
import argparse
import json
import random
import ssl
import sys
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone
from urllib.parse import urlparse, quote

PERFIS = {"sempras-recepcionista": "recepcao", "sempras-tecnico": "tecnico", "sempras-psicologo": "psicologo"}
NOMES = ["Ana Paula Souza", "José Carlos Lima", "Maria das Dores Silva", "Francisco Alves", "Juliana Ribeiro",
         "Antônio Ferreira", "Luciana Martins", "Paulo Henrique Rocha", "Sebastiana Costa", "Raimundo Nonato Dias"]


class Api:
    def __init__(self, base, inseguro):
        self.base = base.rstrip("/")
        self.token = None
        self.ctx = ssl._create_unverified_context() if inseguro else None  # noqa: S323 — CA interna da stack de teste

    def call(self, method, path, body=None, ok=(200, 201)):
        req = urllib.request.Request(self.base + "/" + path.lstrip("/"), method=method,
                                     data=json.dumps(body).encode() if body is not None else None)
        req.add_header("Content-Type", "application/json")
        if self.token:
            req.add_header("Authorization", "Bearer " + self.token)
        try:
            with urllib.request.urlopen(req, context=self.ctx, timeout=30) as r:
                code, raw = r.status, r.read()
        except urllib.error.HTTPError as e:
            code, raw = e.code, e.read()
        data = json.loads(raw) if raw else {}
        if code not in ok:
            raise RuntimeError(f"{method} {path}: HTTP {code} {str(data)[:300]}")
        return code, data.get("data") if isinstance(data, dict) else data

    def login(self, usuario, senha):
        _, d = self.call("POST", "auth/login", {"username": usuario, "password": senha})
        self.token = d["access_token"]


def nis():
    return f"{random.randint(10**10, 10**11 - 1)}"


def garantir_usuario(api, username, nome, senha):
    code, d = api.call("POST", "users", {"username": username, "email": f"{username}@homologacao.local",
                                         "display_name": nome, "password": senha, "roles": ["nexus-user"]}, ok=(201, 409))
    if code == 201:
        return d["id"], True
    _, lista = api.call("GET", f"users?q={quote(username)}")
    for u in lista or []:
        if u["username"] == username:
            return u["id"], False
    raise RuntimeError(f"usuário {username} existe mas não foi encontrado")


def lotar(api, user_id, perfil_id, unidade_id=None, entidade_id=None):
    corpo = {"perfil_id": perfil_id, "principal": True}
    corpo["unidade_id" if unidade_id else "entidade_id"] = unidade_id or entidade_id
    api.call("POST", f"users/{user_id}/lotacoes", corpo, ok=(201, 409, 422))


def povoar(api, u, agora):
    """Cria 3 munícipes na unidade e o que o tipo dela oferta."""
    criados = []
    for i in range(3):
        c = {"nome": random.choice(NOMES), "nis": nis(), "sexo": random.choice(["feminino", "masculino", ""]),
             "data_nascimento": f"{random.randint(1950, 2015)}-0{random.randint(1, 9)}-1{random.randint(0, 9)}",
             "nome_mae": "Mãe de " + random.choice(NOMES).split()[0]}
        _, cid = api.call("POST", "sempras/cidadaos", c)
        criados.append(cid["id"])
    s, uid = set(u["servicos"]), u["id"]
    if "fila" in s and u["demandas"]:
        for cid in criados[:2]:
            api.call("POST", f"sempras/unidades/{uid}/fila",
                     {"cidadao_id": cid, "tipo": "social", "demanda": u["demandas"][0]}, ok=(201, 409))
    c0 = criados[0]
    if "paif" in s:
        api.call("POST", f"sempras/cidadaos/{c0}/paif", {"unidade_id": uid, "tipo_atendimento": "particularizado",
                                                        "bolsa_familia": True})
    if "paefi" in s:
        api.call("POST", f"sempras/cidadaos/{c0}/paefi", {"unidade_id": uid, "violacoes": ["negligencia"]})
    if "centro_pop" in s:
        api.call("POST", f"sempras/unidades/{uid}/abordagens", {
            "descricao_pessoa": "homem, cerca de 40 anos, sob a marquise", "local": "Praça Brasil",
            "data_abordagem": (agora - timedelta(hours=2)).strftime("%Y-%m-%dT%H:%M:%SZ"), "resultado": "orientado"})
    if "casa_mulher" in s:
        api.call("POST", f"sempras/cidadaos/{c0}/casa-mulher", {
            "unidade_id": uid, "violencias": ["psicologica"], "relacao_agressor": "ex_conjuge_companheiro",
            "nivel_risco": "medio", "medida_protetiva_status": "em_analise"})
    if "acolhimento" in s:
        povoar_casa_abrigo(api, uid, agora)
    if "conselho" in s:
        api.call("POST", f"sempras/cidadaos/{c0}/conselho-tutelar", {"unidade_id": uid, "origem_demanda": "escola",
                                                                    "motivo_violacao": "evasao_escolar"})


def dia(d):
    """Data "pura" no formato da API (meia-noite UTC)."""
    return d.strftime("%Y-%m-%dT00:00:00Z")


def povoar_casa_abrigo(api, uid, agora):
    """Duas crianças acolhidas (ADR 032) com uma amostra de cada tela: o
    assistente completo, P.I.A., plano de ação, visita, termo, diário,
    relatório com prazo perto (o monitor avisa) e uma tarefa da equipe."""
    criancas = [("Ana Clara Fictícia", "2015-03-10", "feminino", 40), ("Bruno Fictício", "2011-08-21", "masculino", 85)]
    for nome, nasc, sexo, dias_na_casa in criancas:
        _, c = api.call("POST", "sempras/cidadaos", {"nome": nome, "nis": nis(), "sexo": sexo, "data_nascimento": nasc,
                                                     "nome_mae": "Mãe de " + nome.split()[0]})
        entrada = agora - timedelta(days=dias_na_casa)
        status, a = api.call("POST", f"sempras/cidadaos/{c['id']}/casa-abrigo", {
            "unidade_id": uid, "data_entrada": dia(entrada), "sexo": sexo, "raca_cor": "parda", "filiacao_1": "Mãe de " + nome.split()[0],
            "responsavel_nome": "Tia Fictícia", "responsavel_vinculo": "tia", "responsavel_cep": "78700000",
            "responsavel_logradouro": "Rua Fictícia", "responsavel_numero": "100", "responsavel_cidade": "Rondonópolis", "responsavel_uf": "MT",
            "processo_judicial": f"{random.randint(1000, 9999)}/2026", "guia_acolhimento": f"G-{random.randint(100, 999)}",
            "data_guia": dia(entrada), "historico_acolhido_anteriormente": "nao", "motivos": ["negligencia", "conflito_familiar"],
            "situacao_juridica": "emergencial_conselho_tutelar", "conselheiro_nome": "Conselheira Fictícia", "cna": "nao",
            "estuda": "sim", "recebe_visita": "sim", "relacao_trabalho": "nao",
            "irmaos": [{"nome": "Irmão de " + nome.split()[0], "esta_acolhido": False}],
            "diagnosticos": [{"categoria": "doenca", "diagnostico": "Hipertensão"}] if sexo == "masculino" else [],
        }, ok=(201, 409))
        if status == 409:  # já acolhida numa rodada anterior
            continue
        ac = a["id"]
        api.call("PUT", f"sempras/casa-abrigo/{ac}/pia", {"naturalidade": "Rondonópolis", "escola": "EMEB Fictícia",
                                                         "periodo_escolar": "matutino", "doc_certidao_nascimento": True,
                                                         "intervencao_objetivos": "fortalecer os vínculos com a família extensa"})
        api.call("POST", f"sempras/casa-abrigo/{ac}/pia-metas", {"tipo": "plano", "descricao": "segunda via do RG",
                                                                "responsavel": "Assistente social", "prazo": dia(agora + timedelta(days=5)),
                                                                "esportes": "futsal", "lazer": "parque"})
        api.call("POST", f"sempras/casa-abrigo/{ac}/visitantes", {"nome_visitante": "Tia Fictícia", "parentesco_vinculo": "tia",
                                                                 "data_visita": dia(agora), "hora_entrada": "14:00"})
        api.call("POST", f"sempras/casa-abrigo/{ac}/termos", {"tipo_termo": "passeio", "responsavel_retirada": "Tia Fictícia",
                                                             "parentesco_responsavel": "tia", "data_termo": dia(agora)})
        api.call("POST", f"sempras/unidades/{uid}/casa-abrigo/diario", {"acolhimento_id": ac, "data_referencia": dia(agora),
                                                                       "texto": "Consulta no posto de saúde às 9h."})
    base = f"sempras/unidades/{uid}/casa-abrigo"
    api.call("POST", f"{base}/visitantes", {"tipo_visita": "doacao", "nome_visitante": "Igreja Fictícia", "data_visita": dia(agora),
                                            "hora_entrada": "10:00", "observacoes": "doação de roupas"})
    api.call("POST", f"{base}/diario", {"data_referencia": dia(agora + timedelta(days=2)), "texto": "Reunião da equipe técnica (agenda)."})
    api.call("POST", f"{base}/relatorios", {"tipo": "psicossocial", "status": "em_elaboracao", "data_entrega": dia(agora + timedelta(days=4))})
    api.call("POST", f"{base}/tarefas", {"titulo": "Organizar as doações de inverno", "prioridade": "alta",
                                         "prazo": dia(agora + timedelta(days=2))})


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--api", required=True, help="base da API, p. ex. http://localhost:8002/api/v1")
    p.add_argument("--admin", default="admin")
    p.add_argument("--senha-admin", required=True)
    p.add_argument("--senha", required=True, help="senha dos usuários de homologação criados")
    p.add_argument("--producao-host", default="192.168.1.20", help="host da produção (o script se recusa a usá-lo)")
    p.add_argument("--inseguro", action="store_true", help="aceita a CA interna (HTTPS da stack de teste)")
    a = p.parse_args()
    if urlparse(a.api).hostname == a.producao_host:
        sys.exit(f"Recusado: {a.api} é a produção. Homologação roda numa stack de teste.")
    if len(a.senha) < 10:
        sys.exit("A senha dos usuários de homologação precisa de ao menos 10 caracteres.")

    api = Api(a.api, a.inseguro)
    api.login(a.admin, a.senha_admin)
    _, perfis = api.call("GET", "iam/perfis")
    pid = {pf["slug"]: pf["id"] for pf in perfis}
    _, unidades = api.call("GET", "sempras/unidades/configuracao")
    por_tipo = {}
    for u in sorted(unidades, key=lambda x: x["nome"]):
        if u["tipo"] and u["ativa"]:
            por_tipo.setdefault(u["tipo"], u)
    if not por_tipo:
        sys.exit("Nenhuma unidade com tipo: aplique deploy/sempras/tipos-unidades.sql e ligue o módulo SEMPRAS.")

    agora = datetime.now(timezone.utc)
    linhas = []
    for tipo, u in sorted(por_tipo.items()):
        for slug, papel in PERFIS.items():
            username = f"homolog.{tipo}.{papel}".replace("_", "-")
            uid, novo = garantir_usuario(api, username, f"Homologação {u['nome']} ({papel})", a.senha)
            lotar(api, uid, pid[slug], unidade_id=u["id"])
            linhas.append((username, u["nome"], papel, "criado" if novo else "já existia"))
        povoar(api, u, agora)
    sup, novo = garantir_usuario(api, "homolog.superintendencia", "Homologação Superintendência", a.senha)
    lotar(api, sup, pid["sempras-superintendente"], entidade_id=next(iter(por_tipo.values()))["entidade_id"])
    linhas.append(("homolog.superintendencia", "todas as unidades", "superintendente", "criado" if novo else "já existia"))

    print(f"{'usuário':32} {'unidade':36} {'perfil':16} situação")
    for l in linhas:
        print(f"{l[0]:32} {l[1]:36} {l[2]:16} {l[3]}")
    print(f"\nSenha de todos: a informada em --senha. Dados fictícios criados em {len(por_tipo)} unidades.")


if __name__ == "__main__":
    main()
