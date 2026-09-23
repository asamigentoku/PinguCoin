package config

import "os"

// Config はアプリケーションの設定値を保持する。
type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// InternalAPIToken はpingu-apiからのgRPC呼び出しを検証するための共有シークレット。
	// pingu-api/orcan-api/payment-apiで同じ値を設定する。
	InternalAPIToken string

	// AzureStorageConnectionString は商品画像・ファイルを保存するAzure Blob Storageの接続文字列。
	// デフォルトはAzurite(ローカル開発用エミュレーター)の well-known な接続文字列。
	AzureStorageConnectionString string
	// AzureStoragePublicContainer は商品画像(main_image/sub_images)を保存する共有コンテナ名。
	// Blob単位で匿名読み取りを許可するため、ここに非公開情報を置いてはいけない。
	AzureStoragePublicContainer string
	// AzureStoragePrivateContainer は商品ファイル(販売対象のデジタルコンテンツ)を保存する
	// 共有コンテナ名。匿名アクセス不可(閲覧には都度署名付きURLが必要)。
	AzureStoragePrivateContainer string
	// AppEnv はBlobパスに使う環境名("develop"/"staging"/"production"等)。
	AppEnv string
}

// Load は環境変数から設定を読み込む。未設定の項目にはデフォルト値を使う。
func Load() Config {
	return Config{
		Port:       getEnv("PORT", "8080"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "orcan"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		InternalAPIToken: getEnv("INTERNAL_API_TOKEN", ""),

		// Azuriteの標準開発用アカウント(devstoreaccount1)の well-known な鍵。秘密情報ではない
		// (Azurite/Microsoft公式ドキュメントで公開されている固定値)。
		AzureStorageConnectionString: getEnv("AZURE_STORAGE_CONNECTION_STRING",
			"DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;"+
				"AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;"+
				"BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;"),
		AzureStoragePublicContainer:  getEnv("AZURE_STORAGE_PUBLIC_CONTAINER", "pingue-public"),
		AzureStoragePrivateContainer: getEnv("AZURE_STORAGE_PRIVATE_CONTAINER", "pingue"),
		AppEnv:                       getEnv("APP_ENV", "develop"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
