package repository

import (
	"context"
	"strconv"

	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/cache"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/model"
)

const (
	// キャッシュのキー(Redis のハッシュ)。商品が変わったら、2つとも消す。
	productListHash  = "products:list" // フィールド: 出品者のID(0 = 全件)
	productItemHash  = "products:item" // フィールド: 商品のID
	productCacheName = "products"
)

type ProductRepository struct {
	db *gorm.DB
	// cache は商品の読み取りキャッシュ。nil なら使わない(REDIS_ENABLED=false)。
	cache *cache.Cache
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// WithCache は読み取りキャッシュ(Redis)を有効にする。nil を渡すと、無効のまま。
func (repo *ProductRepository) WithCache(c *cache.Cache) *ProductRepository {
	repo.cache = c
	return repo
}

func (repo *ProductRepository) Create(product *model.Product) error {
	if err := repo.db.Create(product).Error; err != nil {
		return err
	}
	repo.invalidate()
	return nil
}

// FindAll は商品一覧を返す。userID を指定すると出品者で絞り込む。
func (repo *ProductRepository) FindAll(userID uint) ([]model.Product, error) {
	var products []model.Product
	query := repo.db.Preload("Category").Order("id")
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.Find(&products).Error
	return products, err
}

// FindAllCached は FindAll の読み取り専用版で、キャッシュ(Redis)があればそれを返す。
func (repo *ProductRepository) FindAllCached(ctx context.Context, userID uint) ([]model.Product, error) {
	field := strconv.FormatUint(uint64(userID), 10)
	var products []model.Product
	if repo.cache.Get(ctx, productCacheName, productListHash, field, &products) {
		return products, nil
	}
	products, err := repo.FindAll(userID)
	if err != nil {
		return nil, err
	}
	repo.cache.Set(ctx, productListHash, field, products)
	return products, nil
}

func (repo *ProductRepository) FindByID(id uint) (*model.Product, error) {
	var product model.Product
	if err := repo.db.Preload("Category").First(&product, id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

// FindByIDCached は FindByID の読み取り専用版で、キャッシュ(Redis)があればそれを返す。
// 更新の前に読む用途(UpdateProduct など)には使わない: 古い値で上書きしてしまうのを避けるため、
// そちらは FindByID(常に DB)を使う。
func (repo *ProductRepository) FindByIDCached(ctx context.Context, id uint) (*model.Product, error) {
	field := strconv.FormatUint(uint64(id), 10)
	var product model.Product
	if repo.cache.Get(ctx, productCacheName, productItemHash, field, &product) {
		return &product, nil
	}
	found, err := repo.FindByID(id)
	if err != nil {
		return nil, err // 見つからない場合は、キャッシュしない
	}
	repo.cache.Set(ctx, productItemHash, field, found)
	return found, nil
}

func (repo *ProductRepository) Update(product *model.Product) error {
	if err := repo.db.Save(product).Error; err != nil {
		return err
	}
	repo.invalidate()
	return nil
}

func (repo *ProductRepository) Delete(id uint) error {
	if err := repo.db.Delete(&model.Product{}, id).Error; err != nil {
		return err
	}
	repo.invalidate()
	return nil
}

// invalidate は、商品が変わったときに、キャッシュを消す(DB の更新に成功したあとに呼ぶ)。
func (repo *ProductRepository) invalidate() {
	repo.cache.Invalidate(context.Background(), productListHash, productItemHash)
}
