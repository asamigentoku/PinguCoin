-- 認証を Clerk に移したので、不要になった、旧カラム(平文パスワードのハッシュ)を落とす。
-- 残したままだと、NOT NULL 制約により、新規ユーザーの作成(clerk_user_id だけを指定)が失敗する。
-- 新しい DB には、もともと無いので、何も起きない(IF EXISTS)。
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
