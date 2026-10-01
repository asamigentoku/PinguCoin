variable "name" {
  type = string
}

variable "location" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "replication_type" {
  type = string
}

variable "cors_allowed_origins" {
  type = list(string)
}

variable "tags" {
  type = map(string)
}
