package resolvers

import (
	"context"

	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/converters"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph/model"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/apperr"
	orcanpb "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/reqcontext"
)

func (r *mutationResolver) CreateProduct(ctx context.Context, input model.CreateProductInput) (*model.Product, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.Product.CreateProduct(ctx, &orcanpb.CreateProductRequest{
		UserId: uint32(claims.UserID), CategoryId: uint32(input.CategoryID), Name: input.Name,
		Description: converters.StringOrEmpty(input.Description), ImageUrl: converters.StringOrEmpty(input.ImageURL),
		Price: int64(input.Price), Status: converters.StringOrEmpty(input.Status),
	})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.ProductFromPB(response.GetProduct()), nil
}

func (r *mutationResolver) ownedProduct(ctx context.Context, id int32) (*orcanpb.Product, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.Product.GetProduct(ctx, &orcanpb.GetProductRequest{Id: uint32(id)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	if uint(response.GetProduct().GetUserId()) != claims.UserID {
		return nil, apperr.Unauthenticated("cannot modify another user's product")
	}
	return response.GetProduct(), nil
}

func (r *mutationResolver) UpdateProduct(ctx context.Context, id int32, input model.UpdateProductInput) (*model.Product, error) {
	current, err := r.ownedProduct(ctx, id)
	if err != nil {
		return nil, err
	}
	response, err := r.Orcan.Product.UpdateProduct(ctx, &orcanpb.UpdateProductRequest{
		Id: uint32(id), UserId: current.GetUserId(), CategoryId: uint32(input.CategoryID), Name: input.Name,
		Description: converters.StringOrEmpty(input.Description), ImageUrl: current.GetImageUrl(), FileUrl: current.GetFileUrl(),
		Price: int64(input.Price), Status: converters.StringOrEmpty(input.Status),
	})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.ProductFromPB(response.GetProduct()), nil
}

func (r *mutationResolver) DeleteProduct(ctx context.Context, id int32) (bool, error) {
	if _, err := r.ownedProduct(ctx, id); err != nil {
		return false, err
	}
	if _, err := r.Orcan.Product.DeleteProduct(ctx, &orcanpb.DeleteProductRequest{Id: uint32(id)}); err != nil {
		return false, apperr.FromGRPC(err)
	}
	return true, nil
}

func (r *mutationResolver) GetProductImageUploadURL(ctx context.Context, productID int32) (*model.ProductImageUploadTarget, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.Product.GetProductImageUploadURL(ctx, &orcanpb.GetProductImageUploadURLRequest{UserId: uint32(claims.UserID), ProductId: uint32(productID)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return &model.ProductImageUploadTarget{
		BlobEndpoint: response.GetBlobEndpoint(), Container: response.GetContainer(), MainImagePrefix: response.GetMainImagePrefix(),
		SubImagesPrefix: response.GetSubImagesPrefix(), SasToken: response.GetSasToken(), ExpiresAt: converters.FormatTimestamp(response.GetExpiresAt().AsTime()),
	}, nil
}

func (r *mutationResolver) ConfirmProductImageUpload(ctx context.Context, productID int32, fileURL string, description *string, sortOrder *int32) (*model.ConfirmProductImageUploadResult, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	var sortOrderValue int32
	if sortOrder != nil {
		sortOrderValue = *sortOrder
	}
	response, err := r.Orcan.Product.ConfirmProductImageUpload(ctx, &orcanpb.ConfirmProductImageUploadRequest{
		UserId: uint32(claims.UserID), ProductId: uint32(productID), FileUrl: fileURL,
		Description: converters.StringOrEmpty(description), SortOrder: sortOrderValue,
	})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	result := &model.ConfirmProductImageUploadResult{Product: converters.ProductFromPB(response.GetProduct())}
	if response.GetDetailId() != 0 {
		result.Detail = &model.ProductDetail{
			ID: int32(response.GetDetailId()), ProductID: productID, ImageURL: response.GetDetailImageUrl(),
			Description: response.GetDetailDescription(), SortOrder: response.GetDetailSortOrder(),
		}
	}
	return result, nil
}

func (r *mutationResolver) DeleteProductImageUpload(ctx context.Context, productID int32, fileURL string) (*model.Product, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.Product.DeleteProductImageUpload(ctx, &orcanpb.DeleteProductImageUploadRequest{UserId: uint32(claims.UserID), ProductId: uint32(productID), FileUrl: fileURL})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.ProductFromPB(response.GetProduct()), nil
}

func (r *mutationResolver) GetProductFileUploadURL(ctx context.Context, productID int32) (*model.ProductFileUploadTarget, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.Product.GetProductFileUploadURL(ctx, &orcanpb.GetProductFileUploadURLRequest{UserId: uint32(claims.UserID), ProductId: uint32(productID)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return &model.ProductFileUploadTarget{
		BlobEndpoint: response.GetBlobEndpoint(), Container: response.GetContainer(), PathPrefix: response.GetPathPrefix(),
		SasToken: response.GetSasToken(), ExpiresAt: converters.FormatTimestamp(response.GetExpiresAt().AsTime()),
	}, nil
}

func (r *mutationResolver) ConfirmProductFileUpload(ctx context.Context, productID int32, fileURL string) (*model.Product, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.Product.ConfirmProductFileUpload(ctx, &orcanpb.ConfirmProductFileUploadRequest{UserId: uint32(claims.UserID), ProductId: uint32(productID), FileUrl: fileURL})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.ProductFromPB(response.GetProduct()), nil
}

func (r *mutationResolver) DeleteProductFileUpload(ctx context.Context, productID int32, fileURL string) (*model.Product, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.Product.DeleteProductFileUpload(ctx, &orcanpb.DeleteProductFileUploadRequest{UserId: uint32(claims.UserID), ProductId: uint32(productID), FileUrl: fileURL})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.ProductFromPB(response.GetProduct()), nil
}

func (r *mutationResolver) GetProductDownloadURL(ctx context.Context, productID int32) (*model.ProductDownloadTarget, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	productResponse, err := r.Orcan.Product.GetProduct(ctx, &orcanpb.GetProductRequest{Id: uint32(productID)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	if productResponse.GetProduct().GetUserId() != uint32(claims.UserID) {
		purchased, err := r.Orders.HasPaidOrder(claims.UserID, uint(productID))
		if err != nil {
			return nil, apperr.Internal(err)
		}
		if !purchased {
			return nil, apperr.Unauthenticated("product not purchased")
		}
	}
	response, err := r.Orcan.Product.GetProductDownloadURL(ctx, &orcanpb.GetProductDownloadURLRequest{ProductId: uint32(productID)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return &model.ProductDownloadTarget{DownloadURL: response.GetDownloadUrl(), ExpiresAt: converters.FormatTimestamp(response.GetExpiresAt().AsTime())}, nil
}

func (r *queryResolver) Products(ctx context.Context, userID *int32) ([]*model.Product, error) {
	request := &orcanpb.ListProductsRequest{}
	if userID != nil {
		value := uint32(*userID)
		request.UserId = &value
	}
	response, err := r.Orcan.Product.ListProducts(ctx, request)
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	products := make([]*model.Product, 0, len(response.GetProducts()))
	for _, product := range response.GetProducts() {
		products = append(products, converters.ProductFromPB(product))
	}
	return products, nil
}

func (r *queryResolver) Product(ctx context.Context, id int32) (*model.Product, error) {
	response, err := r.Orcan.Product.GetProduct(ctx, &orcanpb.GetProductRequest{Id: uint32(id)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.ProductFromPB(response.GetProduct()), nil
}
