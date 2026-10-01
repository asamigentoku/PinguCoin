package grpcserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
)

func TestValidateProductInput(t *testing.T) {
	tests := []struct {
		name        string
		productName string
		userID      uint32
		categoryID  uint32
		wantErr     string
	}{
		{"valid", "Penguin wallpaper", 1, 1, ""},
		{"empty name", "", 1, 1, "name is required"},
		{"blank name", "   \t", 1, 1, "name is required"},
		{"no seller", "x", 0, 1, "user_id is required"},
		{"no category", "x", 1, 0, "category_id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProductInput(tt.productName, tt.userID, tt.categoryID)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", status.Code(err))
			}
			if appErr, ok := apperr.As(err); !ok || appErr.Message != tt.wantErr {
				t.Errorf("error = %v, want message %q", err, tt.wantErr)
			}
		})
	}
}

func TestMapFindError(t *testing.T) {
	notFound := mapFindError("product", gorm.ErrRecordNotFound)
	if status.Code(notFound) != codes.NotFound {
		t.Errorf("record-not-found should map to NotFound, got %v", status.Code(notFound))
	}
	if appErr, _ := apperr.As(notFound); appErr.Message != "product not found" {
		t.Errorf("message = %q", appErr.Message)
	}

	// DBの生のエラーは内部エラーにし、クライアントには詳細を返さない。
	internal := mapFindError("product", errors.New("connection reset by peer"))
	if status.Code(internal) != codes.Internal {
		t.Errorf("other errors should map to Internal, got %v", status.Code(internal))
	}
	if status.Convert(internal).Message() != "internal server error" {
		t.Errorf("internal error leaks details: %q", status.Convert(internal).Message())
	}
}

func TestStripQuery(t *testing.T) {
	tests := map[string]string{
		"https://acct.blob.core.windows.net/pingue-public/staging/products/1/2/main_image/a.png?sv=2024&sig=SECRET": "https://acct.blob.core.windows.net/pingue-public/staging/products/1/2/main_image/a.png",
		"https://acct.blob.core.windows.net/c/a.png#frag":                                                           "https://acct.blob.core.windows.net/c/a.png",
		"https://acct.blob.core.windows.net/c/a.png":                                                                "https://acct.blob.core.windows.net/c/a.png",
	}
	for input, want := range tests {
		if got := stripQuery(input); got != want {
			t.Errorf("stripQuery(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToProtoProduct(t *testing.T) {
	created := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	product := &model.Product{
		ID: 7, UserID: 3, CategoryID: 2, Name: "Template", Description: "desc",
		ImageURL: "https://img", FileURL: "https://file", Price: 500, Status: "available",
		Version: 4, CreatedAt: created, UpdatedAt: created,
	}

	got := toProtoProduct(product)

	if got.GetId() != 7 || got.GetUserId() != 3 || got.GetCategoryId() != 2 || got.GetName() != "Template" ||
		got.GetPrice() != 500 || got.GetStatus() != "available" || got.GetVersion() != 4 ||
		got.GetImageUrl() != "https://img" || got.GetFileUrl() != "https://file" {
		t.Errorf("unexpected conversion: %+v", got)
	}
	if !got.GetCreatedAt().AsTime().Equal(created) {
		t.Errorf("created_at = %v", got.GetCreatedAt().AsTime())
	}
}

// 入力チェックはDBやストレージに触れる前に行われるので、依存先が無くても検証できる。
func TestServersRejectInvalidInputBeforeTouchingDependencies(t *testing.T) {
	ctx := context.Background()
	products := NewProductServer(nil, nil, nil)
	inventory := NewProductInventoryServer(nil)

	tests := []struct {
		name string
		call func() error
	}{
		{"create product without name", func() error {
			_, err := products.CreateProduct(ctx, &pb.CreateProductRequest{UserId: 1, CategoryId: 1})
			return err
		}},
		{"create product without seller", func() error {
			_, err := products.CreateProduct(ctx, &pb.CreateProductRequest{Name: "x", CategoryId: 1})
			return err
		}},
		{"update product without category", func() error {
			_, err := products.UpdateProduct(ctx, &pb.UpdateProductRequest{Id: 1, Name: "x", UserId: 1})
			return err
		}},
		{"confirm image without url", func() error {
			_, err := products.ConfirmProductImageUpload(ctx, &pb.ConfirmProductImageUploadRequest{UserId: 1, ProductId: 1})
			return err
		}},
		{"delete image without url", func() error {
			_, err := products.DeleteProductImageUpload(ctx, &pb.DeleteProductImageUploadRequest{UserId: 1, ProductId: 1})
			return err
		}},
		{"delete file without url", func() error {
			_, err := products.DeleteProductFileUpload(ctx, &pb.DeleteProductFileUploadRequest{UserId: 1, ProductId: 1})
			return err
		}},
		{"create inventory without product", func() error {
			_, err := inventory.CreateProductInventory(ctx, &pb.CreateProductInventoryRequest{Quantity: 1})
			return err
		}},
		{"create inventory with negative quantity", func() error {
			_, err := inventory.CreateProductInventory(ctx, &pb.CreateProductInventoryRequest{ProductId: 1, Quantity: -1})
			return err
		}},
		{"adjust inventory without product", func() error {
			_, err := inventory.AdjustProductInventory(ctx, &pb.AdjustProductInventoryRequest{Amount: -1, IdempotencyKey: "k"})
			return err
		}},
		{"adjust inventory by zero", func() error {
			_, err := inventory.AdjustProductInventory(ctx, &pb.AdjustProductInventoryRequest{ProductId: 1, IdempotencyKey: "k"})
			return err
		}},
		{"adjust inventory without idempotency key", func() error {
			_, err := inventory.AdjustProductInventory(ctx, &pb.AdjustProductInventoryRequest{ProductId: 1, Amount: -1})
			return err
		}},
		{"adjust inventory with a blank idempotency key", func() error {
			_, err := inventory.AdjustProductInventory(ctx, &pb.AdjustProductInventoryRequest{ProductId: 1, Amount: -1, IdempotencyKey: "   "})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := status.Code(tt.call()); got != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", got)
			}
		})
	}
}
