variable "proxmox_endpoint" {
  description = "URL da API do Proxmox"
  type        = string
  default     = "https://192.168.1.13:8006/"
}

variable "proxmox_api_token" {
  description = "Token no formato user@realm!tokenid=secret (ver deploy/proxmox/README.md)"
  type        = string
  sensitive   = true
}

variable "proxmox_insecure" {
  description = "Aceita o certificado autoassinado do Proxmox (padrão em homelab sem CA própria)"
  type        = bool
  default     = true
}

variable "proxmox_node" {
  description = "Nome do node Proxmox onde a VM é criada"
  type        = string
  default     = "pve"
}

variable "template_vm_id" {
  description = "ID do template cloud-init já criado (deploy/proxmox/create-template.sh)"
  type        = number
  default     = 9000
}

variable "vm_id" {
  description = "ID da VM nova"
  type        = number
  default     = 210
}

variable "vm_name" {
  description = "Nome da VM no Proxmox"
  type        = string
  default     = "projeto-assistencia"
}

variable "vm_ip" {
  description = "IP estático da VM (CIDR é montado com vm_cidr_prefix)"
  type        = string
  default     = "192.168.1.20"
}

variable "vm_cidr_prefix" {
  description = "Prefixo de rede da VM"
  type        = number
  default     = 24
}

variable "vm_gateway" {
  description = "Gateway da rede da VM"
  type        = string
  default     = "192.168.1.1"
}

variable "vm_dns" {
  description = "Servidor DNS da VM"
  type        = string
  default     = "192.168.1.1"
}

variable "vm_bridge" {
  description = "Bridge de rede do Proxmox"
  type        = string
  default     = "vmbr0"
}

variable "vm_storage" {
  description = "Storage do Proxmox para o disco da VM"
  type        = string
  default     = "local-lvm"
}

variable "vm_cores" {
  description = "vCPUs da VM"
  type        = number
  default     = 2
}

variable "vm_memory" {
  description = "RAM da VM em MB"
  type        = number
  default     = 4096
}

variable "vm_disk_size" {
  description = "Disco da VM em GB"
  type        = number
  default     = 40
}

variable "ciuser" {
  description = "Usuário criado pelo cloud-init, usado pelo Ansible para conectar"
  type        = string
  default     = "deploy"
}

variable "ssh_public_key_path" {
  description = "Caminho da chave pública SSH autorizada na VM (sua chave existente)"
  type        = string
}
