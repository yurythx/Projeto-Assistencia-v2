resource "proxmox_virtual_environment_vm" "app" {
  name      = var.vm_name
  node_name = var.proxmox_node
  vm_id     = var.vm_id

  clone {
    vm_id   = var.template_vm_id
    full    = true
    retries = 0
  }

  # Desligado de propósito: a imagem cloud do Ubuntu não traz o
  # qemu-guest-agent instalado, então esperar o ping dele travava a criação
  # por 30min até dar timeout. O Ansible instala e liga o agente depois
  # (útil pro Proxmox, não pro Terraform) — já temos IP fixo via cloud-init,
  # não dependemos do agente pra isso.
  agent {
    enabled = false
  }

  # Depois da criação, o agente é ligado no Proxmox (qm set <id> --agent
  # enabled=1 + desligar/ligar a VM), quando o Ansible já instalou o
  # qemu-guest-agent — o Terraform não deve desligá-lo de novo num apply.
  lifecycle {
    ignore_changes = [agent]
  }

  cpu {
    cores = var.vm_cores
    # "qemu64" (padrão) não tem SSE4.2/POPCNT e quebra imagens Docker que
    # exigem baseline x86-64-v2 (ex.: MinIO — "Fatal glibc error: CPU does
    # not support x86-64-v2"). "host" repassa a CPU física real; ok pra
    # servidor único sem live migration entre hosts com CPUs diferentes.
    type = "host"
  }

  memory {
    dedicated = var.vm_memory
  }

  disk {
    datastore_id = var.vm_storage
    interface    = "scsi0"
    size         = var.vm_disk_size
  }

  network_device {
    bridge = var.vm_bridge
  }

  initialization {
    ip_config {
      ipv4 {
        address = "${var.vm_ip}/${var.vm_cidr_prefix}"
        gateway = var.vm_gateway
      }
    }

    dns {
      servers = [var.vm_dns]
    }

    user_account {
      username = var.ciuser
      keys     = [trimspace(file(pathexpand(var.ssh_public_key_path)))]
    }
  }

  operating_system {
    type = "l26"
  }

  # Cloud-init precisa religar pra aplicar rede/usuário na 1ª vez.
  reboot = true
}
