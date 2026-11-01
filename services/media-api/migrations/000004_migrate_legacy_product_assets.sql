-- 以前の3つの保存の形(products.image_url、product_detail、products.file_url)の参照を、共通のテーブル
-- product_assets にコピーする。storage_url の ON CONFLICT で、すでにあるものは、飛ばす。

-- 用途 1: 商品のメイン画像
INSERT INTO product_assets
    (product_id, purpose_id, storage_url, original_filename, content_type, file_size, description, sort_order, is_primary, metadata, created_at, updated_at)
SELECT id, 1, image_url, '', '', 0, '', 0, TRUE, '{}'::jsonb, created_at, updated_at
FROM products WHERE image_url IS NOT NULL AND image_url <> ''
ON CONFLICT (storage_url) DO NOTHING;

-- 用途 2: 商品の詳細画像
INSERT INTO product_assets
    (product_id, purpose_id, storage_url, original_filename, content_type, file_size, description, sort_order, is_primary, metadata, created_at, updated_at)
SELECT product_id, 2, image_url, '', '', 0, description, sort_order, FALSE, '{}'::jsonb, created_at, updated_at
FROM product_detail WHERE image_url IS NOT NULL AND image_url <> ''
ON CONFLICT (storage_url) DO NOTHING;

-- 用途 3: 販売するファイル
INSERT INTO product_assets
    (product_id, purpose_id, storage_url, original_filename, content_type, file_size, description, sort_order, is_primary, metadata, created_at, updated_at)
SELECT id, 3, file_url, '', '', 0, '', 0, FALSE, '{}'::jsonb, created_at, updated_at
FROM products WHERE file_url IS NOT NULL AND file_url <> ''
ON CONFLICT (storage_url) DO NOTHING;
