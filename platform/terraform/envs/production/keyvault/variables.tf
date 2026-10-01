variable "name" {
  type = string
}

variable "location" {
  type = string
}

variable "resource_group_name" {
  type = string
}

variable "reader_principal_ids" {
  description = "秘密情報を読めるようにするアプリ(サービスプリンシパル)のオブジェクト ID。"
  type        = list(string)
}

variable "secrets" {
  description = "Key Vault に入れる秘密情報(名前 => 値)。名前は小文字・数字・ハイフンだけ。"
  type        = map(string)
  sensitive   = true
}

variable "tags" {
  type = map(string)
}
