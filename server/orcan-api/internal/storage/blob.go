// Package storage は商品の画像・ファイルを保存するAzure Blob Storageへのアクセスをラップする。
//
// コンテナは2つ(環境全体で共有)を使い分ける。
//   - Public: 商品画像(main_image/sub_images)。Blob単位で匿名読み取りを許可するため、
//     アップロード後は署名無しの素のURLでそのまま閲覧できる。
//   - Private: 商品ファイル(販売対象のデジタルコンテンツ)。匿名アクセス不可で、
//     閲覧・ダウンロードのたびにBlob単位の署名付きURL(SAS)が必要。
//
// 商品ごとのパスは "{env}/products/{userID}/{productID}/" というBlob名のプレフィックス
// (仮想フォルダ)で表現する。Azure Blob Storageのコンテナ名はスラッシュを含められず
// 階層も持てないため、商品ごとに実コンテナを分けるのではなく、この方式を採る。
package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// ErrBlobNotInProductPath はfile_urlが対象商品のパス配下(かつ想定コンテナ)を
// 指していない場合のエラー。他の商品/コンテナのファイルを誤って(または悪意を持って)
// 操作させないためのガード。
var ErrBlobNotInProductPath = errors.New("file url is not under the product's path")

const (
	// UploadURLExpiry は発行するアップロード用署名付きURL(SAS)の有効期限。
	UploadURLExpiry = 15 * time.Minute
	// DownloadURLExpiry は発行するダウンロード用署名付きURL(SAS)の有効期限。
	// アップロード用より短くし、ダウンロードのたびに都度発行する運用を前提とする。
	DownloadURLExpiry = 10 * time.Minute

	mainImageSubdir   = "main_image"
	subImagesSubdir   = "sub_images"
	productFileSubdir = "product_file"
)

// ImageKind はBlobパス上での商品画像の種別(メイン画像/サブ画像)を表す。
type ImageKind int

const (
	ImageKindUnknown ImageKind = iota
	ImageKindMain
	ImageKindSub
)

// BlobStorage は商品用の共有コンテナ(公開/非公開)へのアクセスを提供する。
type BlobStorage struct {
	client           *azblob.Client
	publicContainer  string
	privateContainer string
	env              string
}

// New はAzure Blob Storageへの接続を作る。
// connectionString にはAzurite(ローカル開発)または実際のストレージアカウントの接続文字列を渡す。
// publicContainer は商品画像用(匿名読み取り許可)、privateContainer は商品ファイル用(非公開)。
func New(connectionString, publicContainer, privateContainer, env string) (*BlobStorage, error) {
	client, err := azblob.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, err
	}
	return &BlobStorage{
		client:           client,
		publicContainer:  publicContainer,
		privateContainer: privateContainer,
		env:              env,
	}, nil
}

// EnsureContainers は公開/非公開の両コンテナが無ければ作成する(既に存在する場合は何もしない)。
// 商品作成時に呼ばれ、以降は(コンテナが既に存在する限り)実質的に無視されるだけの
// 冪等な呼び出しになる。アップロードURL発行時にも念のため再度呼び、
// 過去に作られた商品に対しても自己修復的に動くようにしている。
func (storage *BlobStorage) EnsureContainers(ctx context.Context) error {
	publicAccess := container.PublicAccessTypeBlob
	if _, err := storage.client.CreateContainer(ctx, storage.publicContainer, &azblob.CreateContainerOptions{Access: &publicAccess}); err != nil &&
		!bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("ensure public container: %w", err)
	}
	if _, err := storage.client.CreateContainer(ctx, storage.privateContainer, nil); err != nil &&
		!bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("ensure private container: %w", err)
	}
	return nil
}

// BlobEndpoint はストレージアカウントのエンドポイント(コンテナ名を含まない、末尾に"/"は付かない)を返す。
// ネットワークアクセスは発生しない(接続文字列から組み立てられたURLを返すだけ)。
// 呼び出し側は BlobEndpoint + "/" + Container + "/" + PathPrefix + <ファイル名> の形で
// 安全に連結できる(二重スラッシュにならない)。
func (storage *BlobStorage) BlobEndpoint() string {
	return strings.TrimSuffix(storage.client.ServiceClient().URL(), "/")
}

// ProductPathPrefix は商品のファイルを置くBlobパスの共通プレフィックスを返す
// (例: "develop/products/5/12/")。
func (storage *BlobStorage) ProductPathPrefix(userID, productID uint32) string {
	return fmt.Sprintf("%s/products/%d/%d/", storage.env, userID, productID)
}

// MainImagePrefix は商品のメイン画像を置くBlobパスのプレフィックスを返す。
func (storage *BlobStorage) MainImagePrefix(userID, productID uint32) string {
	return storage.ProductPathPrefix(userID, productID) + mainImageSubdir + "/"
}

// SubImagesPrefix は商品のサブ画像(複数枚)を置くBlobパスのプレフィックスを返す。
func (storage *BlobStorage) SubImagesPrefix(userID, productID uint32) string {
	return storage.ProductPathPrefix(userID, productID) + subImagesSubdir + "/"
}

// ProductFilePrefix は商品ファイル(デジタルコンテンツ)を置くBlobパスのプレフィックスを返す。
func (storage *BlobStorage) ProductFilePrefix(userID, productID uint32) string {
	return storage.ProductPathPrefix(userID, productID) + productFileSubdir + "/"
}

// UploadURL はコンテナ・パスプレフィックス・署名(SAS)付きアップロード先の情報をまとめて持つ。
type UploadURL struct {
	BlobEndpoint string
	Container    string
	SASToken     string
	ExpiresAt    time.Time
}

// issueContainerUploadURL は指定したコンテナ全体に書き込み可能な署名付きURLを発行する共通処理。
//
// 注意: 標準のBlobコンテナSAS(ADLS Gen2の階層名前空間を使わない場合)はコンテナ単位でのみ
// スコープでき、パスのプレフィックスに限定した署名はできない。そのためこのSASは技術的には
// コンテナ内の他のパスにも書き込み可能であり、「特定のプレフィックス配下にしか書き込まない」
// という制約はクライアント側の実装による取り決めでしかない点に注意する。真にパス単位で
// 権限を絞りたい場合は、ファイルごとのBlob SASを発行する方式や、ADLS Gen2(階層名前空間)の
// 導入を検討すること。
func (storage *BlobStorage) issueContainerUploadURL(ctx context.Context, containerName string) (*UploadURL, error) {
	if err := storage.EnsureContainers(ctx); err != nil {
		return nil, err
	}

	containerClient := storage.client.ServiceClient().NewContainerClient(containerName)
	expiresAt := time.Now().Add(UploadURLExpiry)
	permissions := sas.ContainerPermissions{Read: true, Add: true, Create: true, Write: true, List: true}

	sasURL, err := containerClient.GetSASURL(permissions, expiresAt, nil)
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(sasURL)
	if err != nil {
		return nil, err
	}

	return &UploadURL{
		BlobEndpoint: storage.BlobEndpoint(),
		Container:    containerName,
		SASToken:     parsedURL.RawQuery,
		ExpiresAt:    expiresAt,
	}, nil
}

// IssueImageUploadURL は商品画像(公開コンテナ)へアップロードするための署名付きURLを発行する。
func (storage *BlobStorage) IssueImageUploadURL(ctx context.Context) (*UploadURL, error) {
	return storage.issueContainerUploadURL(ctx, storage.publicContainer)
}

// IssueFileUploadURL は商品ファイル(非公開コンテナ)へアップロードするための署名付きURLを発行する。
func (storage *BlobStorage) IssueFileUploadURL(ctx context.Context) (*UploadURL, error) {
	return storage.issueContainerUploadURL(ctx, storage.privateContainer)
}

// IssueDownloadURL は非公開コンテナ内の指定したBlob1つだけに限定した、読み取り専用の
// 署名付きURLを発行する(コンテナ全体ではなくBlob単位でスコープすることで、
// 他の商品ファイルへのアクセスを防ぐ)。
func (storage *BlobStorage) IssueDownloadURL(ctx context.Context, blobName string) (string, time.Time, error) {
	blobClient := storage.client.ServiceClient().NewContainerClient(storage.privateContainer).NewBlobClient(blobName)
	expiresAt := time.Now().Add(DownloadURLExpiry)

	sasURL, err := blobClient.GetSASURL(sas.BlobPermissions{Read: true}, expiresAt, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	return sasURL, expiresAt, nil
}

// blobNameFromURL は fileURL (SASクエリを含んでいてもよい)から、指定したコンテナ内での
// Blob名を取り出す。他のアカウント/コンテナを指すURLや、指定した商品のパス配下
// (ProductPathPrefix)に無いBlobは ErrBlobNotInProductPath を返し拒否する。
func (storage *BlobStorage) blobNameFromURL(fileURL, containerName string, userID, productID uint32) (string, error) {
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return "", fmt.Errorf("invalid file_url: %w", err)
	}

	endpointURL, err := url.Parse(storage.BlobEndpoint())
	if err != nil {
		return "", err
	}
	if parsedURL.Scheme != endpointURL.Scheme || parsedURL.Host != endpointURL.Host {
		return "", ErrBlobNotInProductPath
	}

	containerPrefix := strings.TrimSuffix(endpointURL.Path, "/") + "/" + containerName + "/"
	blobPathPrefix := containerPrefix + storage.ProductPathPrefix(userID, productID)
	if !strings.HasPrefix(parsedURL.Path, blobPathPrefix) {
		return "", ErrBlobNotInProductPath
	}

	blobName := strings.TrimPrefix(parsedURL.Path, containerPrefix)
	if blobName == "" || strings.HasSuffix(blobName, "/") {
		return "", ErrBlobNotInProductPath
	}
	return blobName, nil
}

// ImageBlobNameFromURL は公開コンテナ(商品画像)内でのBlob名をfile_urlから取り出す。
func (storage *BlobStorage) ImageBlobNameFromURL(fileURL string, userID, productID uint32) (string, error) {
	return storage.blobNameFromURL(fileURL, storage.publicContainer, userID, productID)
}

// FileBlobNameFromURL は非公開コンテナ(商品ファイル)内でのBlob名をfile_urlから取り出す。
func (storage *BlobStorage) FileBlobNameFromURL(fileURL string, userID, productID uint32) (string, error) {
	return storage.blobNameFromURL(fileURL, storage.privateContainer, userID, productID)
}

// ClassifyImageBlobName は(ImageBlobNameFromURLで取り出した)Blob名がmain_image/配下か
// sub_images/配下かを判定する。どちらでもない場合は ImageKindUnknown を返す。
func (storage *BlobStorage) ClassifyImageBlobName(blobName string, userID, productID uint32) ImageKind {
	relative := strings.TrimPrefix(blobName, storage.ProductPathPrefix(userID, productID))
	switch {
	case strings.HasPrefix(relative, mainImageSubdir+"/"):
		return ImageKindMain
	case strings.HasPrefix(relative, subImagesSubdir+"/"):
		return ImageKindSub
	default:
		return ImageKindUnknown
	}
}

// DeleteImageBlob は公開コンテナ内の指定Blobを削除する。既に存在しない場合はエラーにしない(冪等)。
func (storage *BlobStorage) DeleteImageBlob(ctx context.Context, blobName string) error {
	return storage.deleteBlob(ctx, storage.publicContainer, blobName)
}

// DeleteFileBlob は非公開コンテナ内の指定Blobを削除する。既に存在しない場合はエラーにしない(冪等)。
func (storage *BlobStorage) DeleteFileBlob(ctx context.Context, blobName string) error {
	return storage.deleteBlob(ctx, storage.privateContainer, blobName)
}

func (storage *BlobStorage) deleteBlob(ctx context.Context, containerName, blobName string) error {
	containerClient := storage.client.ServiceClient().NewContainerClient(containerName)
	_, err := containerClient.NewBlobClient(blobName).Delete(ctx, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.BlobNotFound) {
		return err
	}
	return nil
}
