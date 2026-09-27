resource "supabase_project" "staging" {
  organization_id   = var.organization_id
  name              = "pingu-staging"
  database_password = var.database_password
  region            = var.region

  lifecycle {
    ignore_changes = [database_password]
  }
}
