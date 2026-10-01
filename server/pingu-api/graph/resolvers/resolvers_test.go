package resolvers

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/model"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/database"
	internalmodel "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/model"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/orcanclient"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/reqcontext"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/testutil"
)

// 偽のorcan-api。必要なメソッドだけ実装し、受け取ったリクエストを記録する。
type fakeProducts struct {
	orcanpb.ProductServiceClient
	products map[uint32]*orcanpb.Product
	created  []*orcanpb.CreateProductRequest
	updated  []*orcanpb.UpdateProductRequest
	deleted  []*orcanpb.DeleteProductRequest
}

func (f *fakeProducts) GetProduct(_ context.Context, in *orcanpb.GetProductRequest, _ ...grpc.CallOption) (*orcanpb.GetProductResponse, error) {
	product, ok := f.products[in.GetId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	return &orcanpb.GetProductResponse{Product: product}, nil
}

func (f *fakeProducts) CreateProduct(_ context.Context, in *orcanpb.CreateProductRequest, _ ...grpc.CallOption) (*orcanpb.CreateProductResponse, error) {
	f.created = append(f.created, in)
	return &orcanpb.CreateProductResponse{Product: &orcanpb.Product{Id: 1, UserId: in.GetUserId(), Name: in.GetName(), CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}}, nil
}

func (f *fakeProducts) UpdateProduct(_ context.Context, in *orcanpb.UpdateProductRequest, _ ...grpc.CallOption) (*orcanpb.UpdateProductResponse, error) {
	f.updated = append(f.updated, in)
	return &orcanpb.UpdateProductResponse{Product: &orcanpb.Product{Id: in.GetId(), UserId: in.GetUserId(), Name: in.GetName(), CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}}, nil
}

func (f *fakeProducts) DeleteProduct(_ context.Context, in *orcanpb.DeleteProductRequest, _ ...grpc.CallOption) (*orcanpb.DeleteProductResponse, error) {
	f.deleted = append(f.deleted, in)
	return &orcanpb.DeleteProductResponse{}, nil
}

type fakeAssets struct {
	orcanpb.ProductAssetServiceClient
	assets   []*orcanpb.ProductAsset
	deleted  []*orcanpb.DeleteProductAssetRequest
	download int
}

func (f *fakeAssets) ListProductAssets(_ context.Context, in *orcanpb.ListProductAssetsRequest, _ ...grpc.CallOption) (*orcanpb.ListProductAssetsResponse, error) {
	var matching []*orcanpb.ProductAsset
	for _, asset := range f.assets {
		if asset.GetProductId() == in.GetProductId() && (in.PurposeId == nil || asset.GetPurposeId() == in.GetPurposeId()) {
			matching = append(matching, asset)
		}
	}
	return &orcanpb.ListProductAssetsResponse{Assets: matching}, nil
}

func (f *fakeAssets) GetProductAsset(_ context.Context, in *orcanpb.GetProductAssetRequest, _ ...grpc.CallOption) (*orcanpb.GetProductAssetResponse, error) {
	for _, asset := range f.assets {
		if asset.GetId() == in.GetId() {
			return &orcanpb.GetProductAssetResponse{Asset: asset}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "asset not found")
}

func (f *fakeAssets) GetProductAssetDownloadURL(_ context.Context, in *orcanpb.GetProductAssetDownloadURLRequest, _ ...grpc.CallOption) (*orcanpb.GetProductAssetDownloadURLResponse, error) {
	f.download++
	return &orcanpb.GetProductAssetDownloadURLResponse{DownloadUrl: "https://blob/signed", ExpiresAt: timestamppb.Now()}, nil
}

func (f *fakeAssets) DeleteProductAsset(_ context.Context, in *orcanpb.DeleteProductAssetRequest, _ ...grpc.CallOption) (*orcanpb.DeleteProductAssetResponse, error) {
	f.deleted = append(f.deleted, in)
	return &orcanpb.DeleteProductAssetResponse{}, nil
}

const (
	seller = 10
	buyer  = 20
	other  = 30
)

type fixture struct {
	mutation *mutationResolver
	query    *queryResolver
	products *fakeProducts
	assets   *fakeAssets
}

func newFixture(orders *repository.OrderRepository) *fixture {
	products := &fakeProducts{products: map[uint32]*orcanpb.Product{
		5: {Id: 5, UserId: seller, CategoryId: 2, Name: "Wallpaper", ImageUrl: "https://blob/img.png", FileUrl: "https://blob/file.zip", Price: 300},
	}}
	assets := &fakeAssets{assets: []*orcanpb.ProductAsset{
		{Id: 1, ProductId: 5, PurposeId: 1, Purpose: &orcanpb.ProductAssetPurpose{Id: 1, Name: "product_image", IsPublic: true}, OriginalFilename: "cover.png"},
		{Id: 2, ProductId: 5, PurposeId: 3, Purpose: &orcanpb.ProductAssetPurpose{Id: 3, Name: "product_file", IsPublic: false}, OriginalFilename: "pack.zip"},
	}}
	resolver := &Resolver{Orcan: &orcanclient.Client{Product: products, Asset: assets}, Orders: orders}
	return &fixture{mutation: &mutationResolver{Resolver: resolver}, query: &queryResolver{Resolver: resolver}, products: products, assets: assets}
}

func loggedInAs(userID uint) context.Context {
	return reqcontext.WithUser(context.Background(), &reqcontext.Claims{UserID: userID})
}

func httpStatus(t *testing.T, err error) int {
	t.Helper()
	appErr, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected an AppError, got %v", err)
	}
	return appErr.HTTPStatus
}

// 出品者は、リクエストの userId ではなく、ログイン中のユーザーに固定される(なりすまし防止)。
func TestCreateProductUsesTheLoggedInUser(t *testing.T) {
	f := newFixture(nil)

	if _, err := f.mutation.CreateProduct(context.Background(), model.CreateProductInput{UserID: seller, CategoryID: 1, Name: "x"}); httpStatus(t, err) != 401 {
		t.Errorf("anonymous create: err = %v, want 401", err)
	}
	if len(f.products.created) != 0 {
		t.Error("an anonymous request reached orcan-api")
	}

	// 別のユーザー(seller)の名前で出品しようとしても、実際の出品者はログイン中の buyer になる。
	if _, err := f.mutation.CreateProduct(loggedInAs(buyer), model.CreateProductInput{UserID: seller, CategoryID: 1, Name: "x", Price: 100}); err != nil {
		t.Fatal(err)
	}
	if got := f.products.created[0].GetUserId(); got != buyer {
		t.Errorf("seller = %d, want the logged-in user %d", got, buyer)
	}
}

func TestUpdateProductIsOwnerOnly(t *testing.T) {
	f := newFixture(nil)
	input := model.UpdateProductInput{UserID: buyer, CategoryID: 2, Name: "Renamed", Price: 999}

	if _, err := f.mutation.UpdateProduct(context.Background(), 5, input); httpStatus(t, err) != 401 {
		t.Errorf("anonymous: err = %v, want 401", err)
	}
	if _, err := f.mutation.UpdateProduct(loggedInAs(other), 5, input); httpStatus(t, err) != 401 {
		t.Errorf("another user: err = %v, want 401", err)
	}
	if len(f.products.updated) != 0 {
		t.Fatal("a non-owner's update reached orcan-api")
	}
	if _, err := f.mutation.UpdateProduct(loggedInAs(seller), 404, input); httpStatus(t, err) != 404 {
		t.Errorf("missing product: err = %v, want 404", err)
	}

	if _, err := f.mutation.UpdateProduct(loggedInAs(seller), 5, input); err != nil {
		t.Fatal(err)
	}
	sent := f.products.updated[0]
	// 所有者は変わらず、画像とファイルのURLは現在の値が引き継がれる(orcan-apiは更新時にこれらを上書きするため)。
	if sent.GetUserId() != seller || sent.GetImageUrl() != "https://blob/img.png" || sent.GetFileUrl() != "https://blob/file.zip" ||
		sent.GetName() != "Renamed" || sent.GetPrice() != 999 {
		t.Errorf("unexpected update request: %+v", sent)
	}
}

func TestDeleteProductIsOwnerOnly(t *testing.T) {
	f := newFixture(nil)

	if _, err := f.mutation.DeleteProduct(loggedInAs(other), 5); httpStatus(t, err) != 401 {
		t.Errorf("another user: err = %v, want 401", err)
	}
	if _, err := f.mutation.DeleteProduct(context.Background(), 5); httpStatus(t, err) != 401 {
		t.Errorf("anonymous: err = %v, want 401", err)
	}
	if len(f.products.deleted) != 0 {
		t.Fatal("a non-owner's delete reached orcan-api")
	}

	ok, err := f.mutation.DeleteProduct(loggedInAs(seller), 5)
	if err != nil || !ok || len(f.products.deleted) != 1 {
		t.Errorf("owner delete: ok=%v err=%v deleted=%d", ok, err, len(f.products.deleted))
	}
}

func filenames(assets []*model.ProductAsset) []string {
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, asset.OriginalFilename)
	}
	return names
}

// 販売するファイル(非公開)の一覧は、出品者本人にだけ返す。
func TestProductAssetsHidePrivateFilesFromAnonymousUsers(t *testing.T) {
	f := newFixture(nil)

	anonymous, err := f.query.ProductAssets(context.Background(), 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if names := filenames(anonymous); len(names) != 1 || names[0] != "cover.png" {
		t.Errorf("anonymous sees %v, want only the public cover.png", names)
	}

	owner, err := f.query.ProductAssets(loggedInAs(seller), 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if names := filenames(owner); len(names) != 2 {
		t.Errorf("the owner sees %v, want both files", names)
	}

	// 用途で絞り込んでも、非公開の用途は未ログインには返らない。
	private := int32(3)
	filtered, _ := f.query.ProductAssets(context.Background(), 5, &private)
	if len(filtered) != 0 {
		t.Errorf("a private purpose filter leaked %v to an anonymous user", filenames(filtered))
	}
}

func TestDownloadRequiresLogin(t *testing.T) {
	f := newFixture(nil)

	if _, err := f.mutation.GetProductAssetDownloadURL(context.Background(), 2); httpStatus(t, err) != 401 {
		t.Errorf("anonymous: err = %v, want 401", err)
	}
	if f.assets.download != 0 {
		t.Error("a signed URL was issued to an anonymous user")
	}
	// 公開ファイルは署名付きURLの対象外(storageUrl をそのまま使う)。
	if _, err := f.mutation.GetProductAssetDownloadURL(loggedInAs(seller), 1); httpStatus(t, err) != 400 {
		t.Errorf("public asset: err = %v, want 400", err)
	}
	if _, err := f.mutation.GetProductAssetDownloadURL(loggedInAs(seller), 999); httpStatus(t, err) != 404 {
		t.Errorf("missing asset: err = %v, want 404", err)
	}

	got, err := f.mutation.GetProductAssetDownloadURL(loggedInAs(seller), 2)
	if err != nil || got.DownloadURL != "https://blob/signed" {
		t.Errorf("the owner should get a download URL: %v %v", got, err)
	}
}

// 出品者以外は、購入済み(決済が完了した注文がある)の場合にだけ、ファイルを見られる・ダウンロードできる。
// 購入の記録はDBにあるので、実際のPostgres(TEST_DATABASE_URL)が必要。
func TestPrivateFilesAreOnlyForTheSellerAndPaidBuyers(t *testing.T) {
	db := testutil.NewDB(t, database.AutoMigrate)
	orders := repository.NewOrderRepository(db)
	if err := orders.Create(&internalmodel.Order{UserID: buyer, ProductID: 5, Quantity: 1, UnitPrice: 300, TotalAmount: 300, PaymentID: 1, Status: "paid", IdempotencyKey: "paid-1"}); err != nil {
		t.Fatal(err)
	}
	// 決済が保留中の注文は、購入済みとは扱わない。
	if err := orders.Create(&internalmodel.Order{UserID: other, ProductID: 5, Quantity: 1, UnitPrice: 300, TotalAmount: 300, PaymentID: 2, Status: "pending", IdempotencyKey: "pending-1"}); err != nil {
		t.Fatal(err)
	}
	f := newFixture(orders)

	for _, tt := range []struct {
		name      string
		user      uint
		canSee    bool
		canGetURL bool
	}{
		{"seller", seller, true, true},
		{"buyer with a paid order", buyer, true, true},
		{"user with only a pending order", other, false, false},
		{"stranger", 99, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assets, err := f.query.ProductAssets(loggedInAs(tt.user), 5, nil)
			if err != nil {
				t.Fatal(err)
			}
			if sees := len(assets) == 2; sees != tt.canSee {
				t.Errorf("sees the private file = %v, want %v (%v)", sees, tt.canSee, filenames(assets))
			}

			_, err = f.mutation.GetProductAssetDownloadURL(loggedInAs(tt.user), 2)
			if got := err == nil; got != tt.canGetURL {
				t.Errorf("can get a download URL = %v, want %v (err=%v)", got, tt.canGetURL, err)
			}
		})
	}
}
