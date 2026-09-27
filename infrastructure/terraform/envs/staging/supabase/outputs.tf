output "project_ref" {
  value = supabase_project.staging.id
}

output "db_host" {
  value = "db.${supabase_project.staging.id}.supabase.co"
}
