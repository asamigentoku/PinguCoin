-- pingu-api: 最初のスキーマ(ベースライン)。
--
-- バージョン管理されたマイグレーションを導入した時点の、全テーブル・インデックス・制約。
--   - 新しい(空の)データベース: このファイルから実行される。
--   - 以前(gorm の AutoMigrate)に作られた既存のデータベース: すでにこのスキーマがあるので、実行せずに「適用済み」と記録する
--     (dbmigrate.Options の SentinelTable で判定する)。
-- このファイルは、適用したあとは、書き換えない(チェックサムで検出され、起動に失敗する)。変更は、新しいファイルで足す。
--
-- 元は、モデル(internal/model)から gorm が作ったスキーマを、pg_dump で書き出したもの。

CREATE TABLE orders (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity bigint NOT NULL,
    unit_price bigint NOT NULL,
    total_amount bigint NOT NULL,
    payment_id bigint NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    idempotency_key character varying(255) NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE SEQUENCE orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE orders_id_seq OWNED BY orders.id;

ALTER TABLE ONLY orders ALTER COLUMN id SET DEFAULT nextval('orders_id_seq'::regclass);

ALTER TABLE ONLY orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);

CREATE UNIQUE INDEX idx_orders_idempotency_key ON orders USING btree (idempotency_key);

CREATE INDEX idx_orders_product_id ON orders USING btree (product_id);

CREATE INDEX idx_orders_user_id ON orders USING btree (user_id);
