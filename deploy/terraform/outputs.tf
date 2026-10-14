output "vm_id" {
  value = proxmox_virtual_environment_vm.app.vm_id
}

output "vm_ip" {
  value = var.vm_ip
}

output "vm_ssh_user" {
  value = var.ciuser
}
