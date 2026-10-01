-- payment-api: 最初のスキーマ(ベースライン)。
--
-- バージョン管理されたマイグレーションを導入した時点の、全テーブル・インデックス・制約。
--   - 新しい(空の)データベース: このファイルから実行される。
--   - 以前(gorm の AutoMigrate)に作られた既存のデータベース: すでにこのスキーマがあるので、実行せずに「適用済み」と記録する
--     (dbmigrate.Options の SentinelTable で判定する)。
-- このファイルは、適用したあとは、書き換えない(チェックサムで検出され、起動に失敗する)。変更は、新しいファイルで足す。
--
-- 元は、モデル(internal/model)から gorm が作ったスキーマを、pg_dump で書き出したもの。

CREATE TABLE payments (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    product_id bigint NOT NULL,
    amount bigint NOT NULL,
    currency character varying(10) NOT NULL,
    payment_method character varying(30) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    idempotency_key character varying(255) NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);

CREATE SEQUENCE payments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE payments_id_seq OWNED BY payments.id;

CREATE TABLE point_accounts (
    user_id bigint NOT NULL,
    balance bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE SEQUENCE point_accounts_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE point_accounts_user_id_seq OWNED BY point_accounts.user_id;

CREATE TABLE point_transactions (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    amount bigint NOT NULL,
    type character varying(20) NOT NULL,
    payment_id bigint,
    reason text,
    balance_after bigint NOT NULL,
    created_at timestamp with time zone
);

CREATE SEQUENCE point_transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE point_transactions_id_seq OWNED BY point_transactions.id;

CREATE TABLE refunds (
    id bigint NOT NULL,
    payment_id bigint NOT NULL,
    amount bigint NOT NULL,
    reason text,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE SEQUENCE refunds_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE refunds_id_seq OWNED BY refunds.id;

ALTER TABLE ONLY payments ALTER COLUMN id SET DEFAULT nextval('payments_id_seq'::regclass);

ALTER TABLE ONLY point_accounts ALTER COLUMN user_id SET DEFAULT nextval('point_accounts_user_id_seq'::regclass);

ALTER TABLE ONLY point_transactions ALTER COLUMN id SET DEFAULT nextval('point_transactions_id_seq'::regclass);

ALTER TABLE ONLY refunds ALTER COLUMN id SET DEFAULT nextval('refunds_id_seq'::regclass);

ALTER TABLE ONLY payments
    ADD CONSTRAINT payments_pkey PRIMARY KEY (id);

ALTER TABLE ONLY point_accounts
    ADD CONSTRAINT point_accounts_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY point_transactions
    ADD CONSTRAINT point_transactions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY refunds
    ADD CONSTRAINT refunds_pkey PRIMARY KEY (id);

CREATE INDEX idx_payments_deleted_at ON payments USING btree (deleted_at);

CREATE UNIQUE INDEX idx_payments_idempotency_key ON payments USING btree (idempotency_key);

CREATE INDEX idx_payments_product_id ON payments USING btree (product_id);

CREATE INDEX idx_payments_user_id ON payments USING btree (user_id);

CREATE INDEX idx_point_transactions_payment_id ON point_transactions USING btree (payment_id);

CREATE INDEX idx_point_transactions_user_id ON point_transactions USING btree (user_id);

CREATE INDEX idx_refunds_payment_id ON refunds USING btree (payment_id);
