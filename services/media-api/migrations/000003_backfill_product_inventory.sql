-- 在庫の行が無い既存の商品に、初期在庫を入れる(購入は、在庫の行が無いとエラーになるため)。
-- 以降の商品は、作成と同じトランザクションで、在庫の行も作られる(model.Product の AfterCreate)。
-- 1000000 は、デジタル商品の初期在庫(model.DefaultDigitalStock。実質、売り切れない)。
INSERT INTO product_inventory (product_id, quantity, reserved, version, created_at, updated_at)
SELECT id, 1000000, 0, 1, NOW(), NOW() FROM products WHERE deleted_at IS NULL
ON CONFLICT (product_id) DO NOTHING;
