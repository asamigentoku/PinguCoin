-- orcan-api: 最初のスキーマ(ベースライン)。
--
-- バージョン管理されたマイグレーションを導入した時点の、全テーブル・インデックス・制約。
--   - 新しい(空の)データベース: このファイルから実行される。
--   - 以前(gorm の AutoMigrate)に作られた既存のデータベース: すでにこのスキーマがあるので、実行せずに「適用済み」と記録する
--     (dbmigrate.Options の SentinelTable で判定する)。
-- このファイルは、適用したあとは、書き換えない(チェックサムで検出され、起動に失敗する)。変更は、新しいファイルで足す。
--
-- 元は、モデル(internal/model)から gorm が作ったスキーマを、pg_dump で書き出したもの。

CREATE TABLE product_asset_purposes (
    id integer NOT NULL,
    name character varying(64) NOT NULL,
    is_public boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE product_assets (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    purpose_id integer NOT NULL,
    storage_url character varying(1024) NOT NULL,
    original_filename character varying(255),
    content_type character varying(255),
    file_size bigint DEFAULT 0 NOT NULL,
    description text,
    sort_order bigint DEFAULT 0 NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE SEQUENCE product_assets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE product_assets_id_seq OWNED BY product_assets.id;

CREATE TABLE product_categories (
    id bigint NOT NULL,
    parent_id bigint,
    name character varying(100) NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);

CREATE SEQUENCE product_categories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE product_categories_id_seq OWNED BY product_categories.id;

CREATE TABLE product_detail (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    image_url character varying(512),
    description text,
    sort_order bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE SEQUENCE product_detail_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE product_detail_id_seq OWNED BY product_detail.id;

CREATE TABLE product_inventory (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity bigint DEFAULT 0 NOT NULL,
    reserved bigint DEFAULT 0 NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE SEQUENCE product_inventory_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE product_inventory_id_seq OWNED BY product_inventory.id;

CREATE TABLE product_inventory_transactions (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    amount bigint NOT NULL,
    reason text,
    idempotency_key character varying(255) NOT NULL,
    quantity_after bigint NOT NULL,
    created_at timestamp with time zone
);

CREATE SEQUENCE product_inventory_transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE product_inventory_transactions_id_seq OWNED BY product_inventory_transactions.id;

CREATE TABLE product_listings (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    price bigint NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    listed_at timestamp with time zone,
    ended_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE SEQUENCE product_listings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE product_listings_id_seq OWNED BY product_listings.id;

CREATE TABLE products (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    category_id bigint NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    image_url character varying(512),
    file_url character varying(512),
    price bigint NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);

CREATE SEQUENCE products_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE products_id_seq OWNED BY products.id;

CREATE TABLE users (
    id bigint NOT NULL,
    clerk_user_id character varying(255) NOT NULL,
    email character varying(255) NOT NULL,
    name character varying(255) NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);

CREATE SEQUENCE users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE users_id_seq OWNED BY users.id;

ALTER TABLE ONLY product_assets ALTER COLUMN id SET DEFAULT nextval('product_assets_id_seq'::regclass);

ALTER TABLE ONLY product_categories ALTER COLUMN id SET DEFAULT nextval('product_categories_id_seq'::regclass);

ALTER TABLE ONLY product_detail ALTER COLUMN id SET DEFAULT nextval('product_detail_id_seq'::regclass);

ALTER TABLE ONLY product_inventory ALTER COLUMN id SET DEFAULT nextval('product_inventory_id_seq'::regclass);

ALTER TABLE ONLY product_inventory_transactions ALTER COLUMN id SET DEFAULT nextval('product_inventory_transactions_id_seq'::regclass);

ALTER TABLE ONLY product_listings ALTER COLUMN id SET DEFAULT nextval('product_listings_id_seq'::regclass);

ALTER TABLE ONLY products ALTER COLUMN id SET DEFAULT nextval('products_id_seq'::regclass);

ALTER TABLE ONLY users ALTER COLUMN id SET DEFAULT nextval('users_id_seq'::regclass);

ALTER TABLE ONLY product_asset_purposes
    ADD CONSTRAINT product_asset_purposes_pkey PRIMARY KEY (id);

ALTER TABLE ONLY product_assets
    ADD CONSTRAINT product_assets_pkey PRIMARY KEY (id);

ALTER TABLE ONLY product_categories
    ADD CONSTRAINT product_categories_pkey PRIMARY KEY (id);

ALTER TABLE ONLY product_detail
    ADD CONSTRAINT product_detail_pkey PRIMARY KEY (id);

ALTER TABLE ONLY product_inventory
    ADD CONSTRAINT product_inventory_pkey PRIMARY KEY (id);

ALTER TABLE ONLY product_inventory_transactions
    ADD CONSTRAINT product_inventory_transactions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY product_listings
    ADD CONSTRAINT product_listings_pkey PRIMARY KEY (id);

ALTER TABLE ONLY products
    ADD CONSTRAINT products_pkey PRIMARY KEY (id);

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

CREATE UNIQUE INDEX idx_product_asset_purposes_name ON product_asset_purposes USING btree (name);

CREATE UNIQUE INDEX idx_product_assets_one_primary ON product_assets USING btree (product_id, purpose_id) WHERE (is_primary = true);

CREATE INDEX idx_product_assets_product_purpose_sort ON product_assets USING btree (product_id, purpose_id, sort_order);

CREATE UNIQUE INDEX idx_product_assets_storage_url ON product_assets USING btree (storage_url);

CREATE INDEX idx_product_categories_deleted_at ON product_categories USING btree (deleted_at);

CREATE INDEX idx_product_categories_parent_id ON product_categories USING btree (parent_id);

CREATE INDEX idx_product_detail_product_id ON product_detail USING btree (product_id);

CREATE UNIQUE INDEX idx_product_inventory_product_id ON product_inventory USING btree (product_id);

CREATE UNIQUE INDEX idx_product_inventory_transactions_idempotency_key ON product_inventory_transactions USING btree (idempotency_key);

CREATE INDEX idx_product_inventory_transactions_product_id ON product_inventory_transactions USING btree (product_id);

CREATE INDEX idx_product_listings_product_id ON product_listings USING btree (product_id);

CREATE INDEX idx_products_category_id ON products USING btree (category_id);

CREATE INDEX idx_products_deleted_at ON products USING btree (deleted_at);

CREATE INDEX idx_products_user_id ON products USING btree (user_id);

CREATE UNIQUE INDEX idx_users_clerk_user_id ON users USING btree (clerk_user_id);

CREATE INDEX idx_users_deleted_at ON users USING btree (deleted_at);

ALTER TABLE ONLY product_assets
    ADD CONSTRAINT fk_product_assets_product FOREIGN KEY (product_id) REFERENCES products(id) ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE ONLY product_assets
    ADD CONSTRAINT fk_product_assets_purpose FOREIGN KEY (purpose_id) REFERENCES product_asset_purposes(id) ON UPDATE CASCADE ON DELETE RESTRICT;

ALTER TABLE ONLY products
    ADD CONSTRAINT fk_products_category FOREIGN KEY (category_id) REFERENCES product_categories(id);
