# Deploy no Proxmox (Terraform + Ansible)

Provisiona uma VM no Proxmox (`192.168.1.13`, node `pve1`) e sobe a stack do
Projeto-Assistencia-v2 nela, chamando os mesmos scripts descritos em
[docs/DEPLOY.md](../../docs/DEPLOY.md) — o Ansible não reimplementa o deploy,
só automatiza "clonar, instalar Docker, rodar `scripts/deploy.sh`".

| Etapa | Ferramenta | O que faz |
|---|---|---|
| 1 | `deploy/proxmox/create-template.sh` | Cria **uma vez** o template cloud-init (Ubuntu 24.04) no Proxmox |
| 2 | `deploy/terraform` | Clona o template numa VM nova, com IP fixo e sua chave SSH |
| 3 | `deploy/ansible` | Instala Docker na VM e roda `scripts/deploy.sh` + `make prod-seed-admin` |

Depois disso a VM é só mais um servidor Docker: `docs/DEPLOY.md` cobre tudo
que vem depois (HTTPS, backup, atualizar com `make deploy`, Keycloak etc.).

## 0. Pré-requisitos nesta máquina

- `terraform` (ou `tofu`) e `ansible` instalados — nenhum dos dois está
  instalado nesta sessão; rode os comandos abaixo de um terminal que tenha
  acesso à rede `192.168.1.0/24` (WSL, outra máquina, ou esta mesma após
  instalar as ferramentas).
- Uma chave SSH sua já existente (ex. `~/.ssh/id_ed25519` /
  `~/.ssh/id_ed25519.pub`) — você escolheu reaproveitar a que já tem, em vez
  de gerar uma nova só para isso.

**Confirme o nome real do node antes de preencher `proxmox_node`** — o prompt
do shell mostra (`root@<node>:~#`); pode não ser `pve`.

## 1. Template cloud-init (uma vez só)

Abra o **Shell** do node na web UI do Proxmox (ou `ssh root@192.168.1.13`) e
rode:

```bash
curl -fsSL -o create-template.sh \
  https://raw.githubusercontent.com/yurythx/projeto-assistencia-v2/main/deploy/proxmox/create-template.sh
chmod +x create-template.sh
./create-template.sh
```

Ou copie o conteúdo de [`create-template.sh`](create-template.sh) manualmente
se o repositório ainda não tiver os commits deste deploy. Cria o template
`9000` (`ubuntu-2404-cloud`) em `local-lvm`, já com o disco em 40G (ver
"Lições aprendidas" abaixo). Idempotente — pode rodar de novo, só não recria
se o VMID já existir. Sem DNS no host (`wget` falha com "Temporary failure in
name resolution")? `echo "nameserver 1.1.1.1" >> /etc/resolv.conf` (teste
antes com `ping -c2 1.1.1.1`: se isso já falhar, o host não tem saída pra
internet mesmo, baixe a imagem em outra máquina e `scp` pro host).

## 2. Token da API (para o Terraform)

Na web UI do Proxmox, ou por SSH no host. **Os nomes de privilégio variam
entre versões do Proxmox** (`VM.Monitor`, por exemplo, não existe em todas) —
mais simples e confiável usar a role embutida `Administrator` num homelab de
nó único:

```bash
pveum user add terraform@pve
pveum aclmod / -user terraform@pve -role Administrator
pveum user token add terraform@pve tf --privsep 0
```

O último comando imprime **uma vez** o `full-tokenid` (algo como
`terraform@pve!tf`) e o `value` (o secret). Junte os dois no formato
`terraform@pve!tf=<value>` — é isso que vai em `proxmox_api_token`.

## 3. Terraform — criar a VM

```bash
cd deploy/terraform
cp terraform.tfvars.example terraform.tfvars
# edite terraform.tfvars: proxmox_api_token (passo 2) e ssh_public_key_path
terraform init
terraform plan   # confere antes de criar algo de verdade
terraform apply
```

Cria a VM `210` (`projeto-assistencia`), clonada do template `9000`, com
`192.168.1.20/24` fixo, 2 vCPU / 4 GB RAM / 40 GB disco, CPU tipo `host`
(ver "Lições aprendidas"), e sua chave pública já autorizada para o usuário
`deploy` (valores em `variables.tf` — ajuste no `.tfvars` se quiser outro
orçamento de recursos ou outro VMID). **Confira `qm list` no host antes de
escolher `vm_id`/`vm_ip`** — não assuma que um ID baixo está livre num
Proxmox que já tem outras VMs.

## 4. Ansible — instalar Docker e subir a stack

```bash
cd deploy/ansible
cp inventory.ini.example inventory.ini
# edite inventory.ini: ansible_ssh_private_key_file e app_public_host se mudou
ansible-playbook playbook.yml
```

Isso instala Docker na VM, clona o repositório, roda
`scripts/deploy.sh --no-pull 192.168.1.20` (gera `.env` com segredos fortes na
1ª vez, builda as imagens, sobe tudo e espera ficar `healthy`) e
`make prod-seed-admin` (cria o admin local — **a senha só aparece uma vez na
saída do Ansible**, guarde-a).

Ao final:

- Frontend: `http://192.168.1.20:3010`
- API: `http://192.168.1.20:8010/health`

## Depois

Tudo em [docs/DEPLOY.md](../../docs/DEPLOY.md) vale a partir daqui:
`scripts/enable-https.sh 192.168.1.20` quando quiser HTTPS,
`scripts/backup.sh --install-cron` para backup diário, `make deploy` (direto
na VM, ou rodando o playbook de novo) para atualizar depois de um `git push`.

## Atualizar depois de mudanças no código

```bash
cd deploy/ansible
ansible-playbook playbook.yml
```

O passo "Clonar ou atualizar o repositório" do playbook já faz o `git pull`
(por isso o `deploy.sh` roda com `--no-pull` — pra não fazer o pull duas
vezes); o restante do playbook builda e sobe a stack de novo.

Mais simples, direto na VM, sem passar pelo Ansible:
`ssh deploy@192.168.1.20 'cd /opt/projeto-assistencia-v2 && sudo make deploy'`.

## Lições aprendidas (deploy real, 2026-10-14)

- **Resize de disco pós-clone quebra o Proxmox.** Pedir no Terraform um
  `disk.size` maior que o disco do template faz o provider chamar um resize
  logo depois do clone — e isso colidiu, na prática, com a escrita do
  `.conf` do clone (`close (rename) atomic file ... failed: File exists`,
  reproduzível em VMIDs diferentes, mesmo após reiniciar `pve-cluster`). O
  `qm clone` direto pela CLI do host nunca falhou — só a chamada da API via
  Terraform. Fix definitivo: `qm resize <template> scsi0 40G` **no
  template, antes de virar template** (já no `create-template.sh`), e
  `disk.size` no Terraform igual ao do template — sem resize, sem race.
- **`agent.enabled = true` trava a criação por até 30min.** A imagem cloud
  oficial do Ubuntu não traz `qemu-guest-agent` instalado. Com IP fixo via
  cloud-init não precisamos do agente pra isso — `agent.enabled = false` no
  `main.tf`. O Ansible instala e liga o agente depois (bom pro Proxmox:
  shutdown limpo, snapshot consistente), só não bloqueia a criação da VM.
- **CPU padrão (`qemu64`) quebra imagens que exigem x86-64-v2.** A imagem
  `pgsty/minio` do `docker-compose.yml` crasha com `Fatal glibc error: CPU
  does not support x86-64-v2` nesse CPU model genérico — Postgres/Redis/Go/
  Node não têm esse requisito, só o MinIO. Fix: `cpu.type = "host"` (ok pra
  servidor único; evite em cluster com hosts de CPU diferente, por causa de
  live migration).
- **CPU, ao contrário do IP/disco do cloud-init, só aplica depois de um
  reboot da VM** (`qm reboot <vmid>`) — um `terraform apply` que só muda
  `cpu.type` conclui rápido mas não reinicia a VM sozinho.
