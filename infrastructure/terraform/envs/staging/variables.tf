variable "organization_id" {
  type = string
}

variable "database_password" {
  type      = string
  sensitive = true
}

variable "region" {
  type    = string
  default = "ap-northeast-1"
}
