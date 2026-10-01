variable "name_prefix" {
  type = string
}

variable "location" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "subnet_id" {
  type = string
}

variable "log_analytics_workspace_id" {
  type = string
}

variable "kubernetes_version" {
  type    = string
  default = null
}

variable "node_vm_size" {
  type = string
}

variable "node_count" {
  type = number
}

variable "sku_tier" {
  type = string
}

variable "enable_container_insights" {
  type = bool
}

variable "admin_group_object_ids" {
  type = list(string)
}

variable "tags" {
  type = map(string)
}
