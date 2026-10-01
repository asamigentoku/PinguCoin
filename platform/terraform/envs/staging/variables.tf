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

variable "azure_subscription_id" {
  description = "Azure subscription ID used for staging resources."
  type        = string
}

variable "azure_location" {
  description = "Azure region for staging resources."
  type        = string
  default     = "Japan East"
}

variable "azure_resource_group_name" {
  description = "Resource group that contains the staging Azure resources."
  type        = string
  default     = "pingucoin-staging-rg"
}

variable "azure_storage_account_name" {
  description = "Globally unique Storage Account name (3-24 lowercase letters and numbers)."
  type        = string

  validation {
    condition = (
      length(var.azure_storage_account_name) >= 3 &&
      length(var.azure_storage_account_name) <= 24 &&
      can(regex("^[a-z0-9]+$", var.azure_storage_account_name))
    )
    error_message = "azure_storage_account_name must contain 3-24 lowercase letters and numbers."
  }
}

variable "storage_cors_allowed_origins" {
  description = "Origins allowed to upload directly to Azure Blob Storage. Do not include a trailing slash."
  type        = list(string)

  validation {
    condition     = length(var.storage_cors_allowed_origins) > 0
    error_message = "storage_cors_allowed_origins must contain at least one origin."
  }
}
