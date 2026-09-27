module "supabase" {
  source = "./supabase"

  organization_id   = var.organization_id
  database_password = var.database_password
  region            = var.region

  providers = {
    supabase = supabase
  }
}

module "azure_blob" {
  source = "./azure-blob"

  location             = var.azure_location
  resource_group_name  = var.azure_resource_group_name
  storage_account_name = var.azure_storage_account_name
  cors_allowed_origins = var.storage_cors_allowed_origins

  providers = {
    azurerm = azurerm
  }
}

# Preserve the existing Supabase project while changing its Terraform address.
moved {
  from = supabase_project.staging
  to   = module.supabase.supabase_project.staging
}
