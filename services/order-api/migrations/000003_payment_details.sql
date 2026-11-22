-- 支払方法、通貨、追加認証URLなど、決済連携に必要な注文情報を追加する。
ALTER TABLE orders ADD COLUMN IF NOT EXISTS payment_method varchar(30) NOT NULL DEFAULT 'point';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS currency varchar(10) NOT NULL DEFAULT 'POINT';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS action_url text NOT NULL DEFAULT '';
