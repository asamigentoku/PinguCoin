terraform {
  required_providers {
    supabase = {
      source  = "supabase/supabase"
      version = "~> 1.0"
    }
  }
}

# アクセストークンは環境変数 SUPABASE_ACCESS_TOKEN から読み込む
provider "supabase" {}
