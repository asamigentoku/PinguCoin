package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/asamigentoku/PinguCoin/services/bff/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orderclient"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/repository"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/reqcontext"
	media "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CatalogHandler struct {
	media        *orcanclient.Client
	orderAPI     *orderclient.Client
	legacyOrders *repository.OrderRepository
}

func NewCatalogHandler(client *orcanclient.Client, orderAPI *orderclient.Client, legacyOrders *repository.OrderRepository) *CatalogHandler {
	return &CatalogHandler{media: client, orderAPI: orderAPI, legacyOrders: legacyOrders}
}

type productResponse struct {
	ID          uint32 `json:"id"`
	UserID      uint32 `json:"userId"`
	CategoryID  uint32 `json:"categoryId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	FileURL     string `json:"fileUrl"`
	Price       int64  `json:"price"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	Version     uint32 `json:"version"`
}
type assetResponse struct {
	ID               uint32 `json:"id"`
	ProductID        uint32 `json:"productId"`
	PurposeID        uint32 `json:"purposeId"`
	StorageURL       string `json:"storageUrl"`
	OriginalFilename string `json:"originalFilename"`
	ContentType      string `json:"contentType"`
	FileSize         int64  `json:"fileSize"`
	Description      string `json:"description"`
	SortOrder        int32  `json:"sortOrder"`
	IsPrimary        bool   `json:"isPrimary"`
	Metadata         string `json:"metadata"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}
type productInput struct {
	CategoryID  uint32 `json:"categoryId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	Status      string `json:"status"`
}
type imageInput struct {
	FileURL     string `json:"fileUrl"`
	Description string `json:"description"`
	SortOrder   int32  `json:"sortOrder"`
}
type assetInput struct {
	PurposeID        uint32 `json:"purposeId"`
	FileURL          string `json:"fileUrl"`
	OriginalFilename string `json:"originalFilename"`
	ContentType      string `json:"contentType"`
	FileSize         int64  `json:"fileSize"`
	Description      string `json:"description"`
	SortOrder        int32  `json:"sortOrder"`
	IsPrimary        bool   `json:"isPrimary"`
	Metadata         string `json:"metadata"`
}

func timestamp(value *timestamppb.Timestamp) string {
	if value == nil {
		return ""
	}
	return value.AsTime().Format(time.RFC3339)
}
func productJSON(v *media.Product) *productResponse {
	if v == nil {
		return nil
	}
	return &productResponse{v.GetId(), v.GetUserId(), v.GetCategoryId(), v.GetName(), v.GetDescription(), v.GetImageUrl(), v.GetFileUrl(), v.GetPrice(), v.GetStatus(), timestamp(v.GetCreatedAt()), timestamp(v.GetUpdatedAt()), v.GetVersion()}
}
func assetJSON(v *media.ProductAsset) *assetResponse {
	if v == nil {
		return nil
	}
	return &assetResponse{v.GetId(), v.GetProductId(), v.GetPurposeId(), v.GetStorageUrl(), v.GetOriginalFilename(), v.GetContentType(), v.GetFileSize(), v.GetDescription(), v.GetSortOrder(), v.GetIsPrimary(), v.GetMetadata(), timestamp(v.GetCreatedAt()), timestamp(v.GetUpdatedAt())}
}
func pathID(r *http.Request) (uint32, error) {
	v, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || v == 0 {
		return 0, apperr.InvalidArgument("invalid id")
	}
	return uint32(v), nil
}
func queryID(r *http.Request, key string) (*uint32, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return nil, apperr.InvalidArgument("invalid " + key)
	}
	result := uint32(v)
	return &result, nil
}
func decodeBody(r *http.Request, target any) error {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return apperr.InvalidArgument("invalid JSON body")
	}
	return nil
}
func authenticated(r *http.Request) (*reqcontext.Claims, error) {
	v, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	return v, nil
}

func (h *CatalogHandler) Me(w http.ResponseWriter, r *http.Request) {
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": u.UserID, "email": u.Email, "name": u.Name})
}
func (h *CatalogHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	userID, err := queryID(r, "userId")
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.listProducts(w, r, userID)
}
func (h *CatalogHandler) ListMyProducts(w http.ResponseWriter, r *http.Request) {
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := uint32(u.UserID)
	h.listProducts(w, r, &id)
}
func (h *CatalogHandler) listProducts(w http.ResponseWriter, r *http.Request, userID *uint32) {
	response, err := h.media.Product.ListProducts(r.Context(), &media.ListProductsRequest{UserId: userID})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	result := make([]*productResponse, 0, len(response.GetProducts()))
	for _, v := range response.GetProducts() {
		result = append(result, productJSON(v))
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *CatalogHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	response, err := h.media.Product.GetProduct(r.Context(), &media.GetProductRequest{Id: id})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	writeJSON(w, http.StatusOK, productJSON(response.GetProduct()))
}
func (h *CatalogHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input productInput
	if err = decodeBody(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	response, err := h.media.Product.CreateProduct(r.Context(), &media.CreateProductRequest{UserId: uint32(u.UserID), CategoryId: input.CategoryID, Name: input.Name, Description: input.Description, Price: input.Price, Status: input.Status})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	writeJSON(w, http.StatusCreated, productJSON(response.GetProduct()))
}
func (h *CatalogHandler) owned(r *http.Request, id uint32) (*media.Product, *reqcontext.Claims, error) {
	u, err := authenticated(r)
	if err != nil {
		return nil, nil, err
	}
	response, err := h.media.Product.GetProduct(r.Context(), &media.GetProductRequest{Id: id})
	if err != nil {
		return nil, nil, apperr.FromGRPC(err)
	}
	if response.GetProduct().GetUserId() != uint32(u.UserID) {
		return nil, nil, apperr.PermissionDenied("cannot modify another user's product")
	}
	return response.GetProduct(), u, nil
}
func (h *CatalogHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	current, _, err := h.owned(r, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input productInput
	if err = decodeBody(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	response, err := h.media.Product.UpdateProduct(r.Context(), &media.UpdateProductRequest{Id: id, UserId: current.GetUserId(), CategoryId: input.CategoryID, Name: input.Name, Description: input.Description, ImageUrl: current.GetImageUrl(), FileUrl: current.GetFileUrl(), Price: input.Price, Status: input.Status})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	writeJSON(w, http.StatusOK, productJSON(response.GetProduct()))
}
func (h *CatalogHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if _, _, err = h.owned(r, id); err != nil {
		writeError(w, r, err)
		return
	}
	if _, err = h.media.Product.DeleteProduct(r.Context(), &media.DeleteProductRequest{Id: id}); err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CatalogHandler) GetUploadTargets(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	purpose, err := queryID(r, "purposeId")
	if err != nil || purpose == nil {
		if err == nil {
			err = apperr.InvalidArgument("purposeId is required")
		}
		writeError(w, r, err)
		return
	}
	image, err := h.media.Product.GetProductImageUploadURL(r.Context(), &media.GetProductImageUploadURLRequest{UserId: uint32(u.UserID), ProductId: id})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	file, err := h.media.Asset.GetProductAssetUploadURL(r.Context(), &media.GetProductAssetUploadURLRequest{UserId: uint32(u.UserID), ProductId: id, PurposeId: *purpose})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"image": map[string]any{"blobEndpoint": image.GetBlobEndpoint(), "container": image.GetContainer(), "mainImagePrefix": image.GetMainImagePrefix(), "subImagesPrefix": image.GetSubImagesPrefix(), "sasToken": image.GetSasToken(), "expiresAt": timestamp(image.GetExpiresAt())}, "files": map[string]any{"blobEndpoint": file.GetBlobEndpoint(), "container": file.GetContainer(), "pathPrefix": file.GetPathPrefix(), "sasToken": file.GetSasToken(), "expiresAt": timestamp(file.GetExpiresAt())}})
}
func (h *CatalogHandler) ConfirmImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input imageInput
	if err = decodeBody(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	response, err := h.media.Product.ConfirmProductImageUpload(r.Context(), &media.ConfirmProductImageUploadRequest{UserId: uint32(u.UserID), ProductId: id, FileUrl: input.FileURL, Description: input.Description, SortOrder: input.SortOrder})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	writeJSON(w, http.StatusOK, productJSON(response.GetProduct()))
}
func (h *CatalogHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input imageInput
	if err = decodeBody(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	if _, err = h.media.Product.DeleteProductImageUpload(r.Context(), &media.DeleteProductImageUploadRequest{UserId: uint32(u.UserID), ProductId: id, FileUrl: input.FileURL}); err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *CatalogHandler) ConfirmAsset(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input assetInput
	if err = decodeBody(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	response, err := h.media.Asset.ConfirmProductAssetUpload(r.Context(), &media.ConfirmProductAssetUploadRequest{UserId: uint32(u.UserID), ProductId: id, PurposeId: input.PurposeID, FileUrl: input.FileURL, OriginalFilename: input.OriginalFilename, ContentType: input.ContentType, FileSize: input.FileSize, Description: input.Description, SortOrder: input.SortOrder, IsPrimary: input.IsPrimary, Metadata: input.Metadata})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	writeJSON(w, http.StatusCreated, assetJSON(response.GetAsset()))
}
func (h *CatalogHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if _, err = h.media.Asset.DeleteProductAsset(r.Context(), &media.DeleteProductAssetRequest{UserId: uint32(u.UserID), AssetId: id}); err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CatalogHandler) hasPaidOrder(ctx context.Context, userID uint, productID uint) (bool, error) {
	if h.orderAPI != nil {
		response, err := h.orderAPI.HasPaidOrder(ctx, uint64(userID), uint64(productID))
		if err != nil {
			return false, apperr.FromGRPC(err)
		}
		return response, nil
	}
	if h.legacyOrders != nil {
		paid, err := h.legacyOrders.HasPaidOrder(userID, productID)
		if err != nil {
			return false, apperr.Internal(err)
		}
		return paid, nil
	}
	return false, nil
}
func (h *CatalogHandler) canSeePrivate(r *http.Request, productID uint32) (bool, error) {
	u, ok := reqcontext.UserFromContext(r.Context())
	if !ok {
		return false, nil
	}
	response, err := h.media.Product.GetProduct(r.Context(), &media.GetProductRequest{Id: productID})
	if err != nil {
		return false, apperr.FromGRPC(err)
	}
	if response.GetProduct().GetUserId() == uint32(u.UserID) {
		return true, nil
	}
	return h.hasPaidOrder(r.Context(), u.UserID, uint(productID))
}
func (h *CatalogHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	purpose, err := queryID(r, "purposeId")
	if err != nil {
		writeError(w, r, err)
		return
	}
	response, err := h.media.Asset.ListProductAssets(r.Context(), &media.ListProductAssetsRequest{ProductId: id, PurposeId: purpose})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	allowed, err := h.canSeePrivate(r, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	result := make([]*assetResponse, 0, len(response.GetAssets()))
	for _, v := range response.GetAssets() {
		if v.GetPurpose().GetIsPublic() || allowed {
			result = append(result, assetJSON(v))
		}
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *CatalogHandler) GetAssetDownload(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	u, err := authenticated(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	assetResult, err := h.media.Asset.GetProductAsset(r.Context(), &media.GetProductAssetRequest{Id: id})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	asset := assetResult.GetAsset()
	if asset.GetPurpose().GetIsPublic() {
		writeError(w, r, apperr.InvalidArgument("public asset uses storageUrl"))
		return
	}
	productResult, err := h.media.Product.GetProduct(r.Context(), &media.GetProductRequest{Id: asset.GetProductId()})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	if productResult.GetProduct().GetUserId() != uint32(u.UserID) {
		paid, paidErr := h.hasPaidOrder(r.Context(), u.UserID, uint(asset.GetProductId()))
		if paidErr != nil {
			writeError(w, r, paidErr)
			return
		}
		if !paid {
			writeError(w, r, apperr.PermissionDenied("product not purchased"))
			return
		}
	}
	response, err := h.media.Asset.GetProductAssetDownloadURL(r.Context(), &media.GetProductAssetDownloadURLRequest{AssetId: id})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"downloadUrl": response.GetDownloadUrl(), "expiresAt": timestamp(response.GetExpiresAt())})
}
