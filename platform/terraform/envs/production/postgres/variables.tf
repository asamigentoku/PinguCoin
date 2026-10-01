variable "name" {
  type = string
}

variable "location" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "delegated_subnet_id" {
  type = string
}

variable "private_dns_zone_id" {
  type = string
}

variable "private_dns_zone_link_id" {
  type = string
}

variable "sku_name" {
  type = string
}

variable "storage_mb" {
  type = number
}

variable "high_availability" {
  type = string
}

variable "backup_retention_days" {
  type = number
}

variable "geo_redundant_backup" {
  type = bool
}

variable "tags" {
  type = map(string)
}
