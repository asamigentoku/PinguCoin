-- 参照データ(マスタ)の初期値。すでにある行は、変更しない(ON CONFLICT DO NOTHING)。

-- 商品アセットの用途。用途は、DB のデータとして持つ(列挙型や CHECK 制約にしない)ので、
-- 用途を増やしても、product_assets のスキーマは変えなくてよい。
INSERT INTO product_asset_purposes (id, name, is_public, created_at, updated_at) VALUES
    (1, 'product_image', TRUE, now(), now()),
    (2, 'detail_image', TRUE, now(), now()),
    (3, 'product_file', FALSE, now(), now())
ON CONFLICT DO NOTHING;

-- カテゴリー。client-web の lib/categories.ts と同じ ID・名前。
-- 商品は category_id の外部キーを持つので、この行が無いと、出品(商品の作成)が制約違反で失敗する。
INSERT INTO product_categories (id, name, created_at, updated_at) VALUES
    (1, 'アート・イラスト', now(), now()),
    (2, 'テンプレート', now(), now()),
    (3, '音楽・サウンド', now(), now()),
    (4, '便利ツール', now(), now())
ON CONFLICT DO NOTHING;

-- ID を明示して入れると、シーケンスが進まない。以降に採番する ID(カテゴリーの追加)が、上の ID と衝突しないよう、
-- シーケンスを、最大の ID まで進める。
SELECT setval(pg_get_serial_sequence('product_categories', 'id'), (SELECT COALESCE(MAX(id), 1) FROM product_categories));
