package orcanclient

import (
	pb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
)

// RetryableMethods は、一時的な失敗(UNAVAILABLE)のときに、再試行してよい orcan-api のメソッド。
// 生成コードの定数を使っているので、メソッド名の書き間違いは、ビルドエラーになる。
//
// 再試行してよいのは、同じ呼び出しを2回やっても結果が変わらない(冪等な)ものだけ。
//   - 読み取り(Get / List、ダウンロード用URLの発行)
//   - 冪等性キーが付いている書き込み(AdjustProductInventory)、find-or-create の EnsureUser
//
// 次のような「2回やると結果が変わる」書き込みは、入れてはいけない(retryable_test.go が確かめる)。
//
//	Create* / Update* / Delete* / Confirm*(商品・カテゴリー・詳細・在庫・出品・アセットの作成・更新・削除・確定)
var RetryableMethods = []string{
	// 商品
	pb.ProductService_ListProducts_FullMethodName,
	pb.ProductService_GetProduct_FullMethodName,
	pb.ProductService_GetProductDownloadURL_FullMethodName,
	// アセット(画像・販売ファイル)
	pb.ProductAssetService_ListProductAssets_FullMethodName,
	pb.ProductAssetService_GetProductAsset_FullMethodName,
	pb.ProductAssetService_GetProductAssetDownloadURL_FullMethodName,
	// カテゴリー・詳細・出品
	pb.ProductCategoryService_ListProductCategories_FullMethodName,
	pb.ProductCategoryService_GetProductCategory_FullMethodName,
	pb.ProductDetailService_ListProductDetails_FullMethodName,
	pb.ProductDetailService_GetProductDetail_FullMethodName,
	pb.ProductListingService_ListProductListings_FullMethodName,
	pb.ProductListingService_GetProductListing_FullMethodName,
	// 在庫(AdjustProductInventory は、冪等性キーで、同じキーの再送を二重に減らさない)
	pb.ProductInventoryService_ListProductInventories_FullMethodName,
	pb.ProductInventoryService_GetProductInventory_FullMethodName,
	pb.ProductInventoryService_AdjustProductInventory_FullMethodName,
	// ユーザー(EnsureUser は、Clerk のIDで、あれば返し、なければ作る)
	pb.UserService_ListUsers_FullMethodName,
	pb.UserService_GetUser_FullMethodName,
	pb.UserService_GetUserByClerkID_FullMethodName,
	pb.UserService_EnsureUser_FullMethodName,
}
