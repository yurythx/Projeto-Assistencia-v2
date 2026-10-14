#!/usr/bin/env bash
# Roda UMA VEZ no host Proxmox (shell da web UI no node, ou `ssh root@192.168.1.13`)
# para criar o template cloud-init que o Terraform (deploy/terraform) clona a
# cada VM. Idempotente: pode rodar de novo, mas só recria se o VMID não existir.
#
#   VMID=9000 STORAGE=local-lvm ./create-template.sh
set -euo pipefail

VMID="${VMID:-9000}"
STORAGE="${STORAGE:-local-lvm}"
BRIDGE="${BRIDGE:-vmbr0}"
IMG=noble-server-cloudimg-amd64.img
URL="https://cloud-images.ubuntu.com/noble/current/$IMG"

if qm status "$VMID" >/dev/null 2>&1; then
  echo "VMID $VMID já existe (template ou VM) — nada a fazer." >&2
  exit 0
fi

cd /var/lib/vz/template/iso
[ -f "$IMG" ] || wget -O "$IMG" "$URL"

qm create "$VMID" --name ubuntu-2404-cloud --memory 1024 --cores 1 --net0 "virtio,bridge=$BRIDGE"
qm importdisk "$VMID" "$IMG" "$STORAGE"
qm set "$VMID" --scsihw virtio-scsi-pci --scsi0 "$STORAGE:vm-$VMID-disk-0"
qm set "$VMID" --ide2 "$STORAGE:cloudinit"
qm set "$VMID" --boot order=scsi0
qm set "$VMID" --serial0 socket --vga serial0
qm set "$VMID" --agent enabled=1
# Redimensiona ANTES de virar template: um full clone copia o disco no
# tamanho atual do template. Se o Terraform pedisse resize depois do clone
# (3.5G -> 40G), isso colidia com a escrita do .conf do clone (bug visto
# na prática: "close (rename) atomic file ... failed: File exists").
# Combine com disk.size no deploy/terraform/main.tf (padrão 40G).
qm resize "$VMID" scsi0 "${DISK_SIZE:-40}G"
qm template "$VMID"

echo "Template $VMID (ubuntu-2404-cloud) criado em $STORAGE."
