package storage

import (
	"strings"
	"testing"
)

// devstoreaccount1の well-known な接続文字列(Azuriteのローカル開発用固定値。秘密情報ではない)。
const testConnectionString = "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;" +
	"AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;" +
	"BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;"

const (
	testPublicContainer  = "pingue-public"
	testPrivateContainer = "pingue"
)

func newTestStorage(t *testing.T) *BlobStorage {
	t.Helper()
	s, err := New(testConnectionString, testPublicContainer, testPrivateContainer, "develop")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return s
}

func TestProductPathPrefix(t *testing.T) {
	s := newTestStorage(t)

	if got, want := s.ProductPathPrefix(5, 12), "develop/products/5/12/"; got != want {
		t.Errorf("ProductPathPrefix() = %q, want %q", got, want)
	}
	if got, want := s.MainImagePrefix(5, 12), "develop/products/5/12/main_image/"; got != want {
		t.Errorf("MainImagePrefix() = %q, want %q", got, want)
	}
	if got, want := s.SubImagesPrefix(5, 12), "develop/products/5/12/sub_images/"; got != want {
		t.Errorf("SubImagesPrefix() = %q, want %q", got, want)
	}
	if got, want := s.ProductFilePrefix(5, 12), "develop/products/5/12/product_file/"; got != want {
		t.Errorf("ProductFilePrefix() = %q, want %q", got, want)
	}
}

func TestImageBlobNameFromURL_Valid(t *testing.T) {
	s := newTestStorage(t)
	endpoint := strings.TrimSuffix(s.BlobEndpoint(), "/")
	base := endpoint + "/" + testPublicContainer + "/"

	cases := []struct {
		name string
		url  string
		want string
		kind ImageKind
	}{
		{"main image, no query", base + s.MainImagePrefix(5, 12) + "photo.jpg", "develop/products/5/12/main_image/photo.jpg", ImageKindMain},
		{"main image, with SAS query", base + s.MainImagePrefix(5, 12) + "photo.jpg?sv=2023&sig=abc", "develop/products/5/12/main_image/photo.jpg", ImageKindMain},
		{"sub image, nested path", base + s.SubImagesPrefix(5, 12) + "a/b.jpg", "develop/products/5/12/sub_images/a/b.jpg", ImageKindSub},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ImageBlobNameFromURL(tc.url, 5, 12)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ImageBlobNameFromURL() = %q, want %q", got, tc.want)
			}
			if kind := s.ClassifyImageBlobName(got, 5, 12); kind != tc.kind {
				t.Errorf("ClassifyImageBlobName() = %v, want %v", kind, tc.kind)
			}
		})
	}
}

func TestImageBlobNameFromURL_Rejected(t *testing.T) {
	s := newTestStorage(t)
	endpoint := strings.TrimSuffix(s.BlobEndpoint(), "/")
	base := endpoint + "/" + testPublicContainer + "/"

	cases := []struct {
		name string
		url  string
	}{
		{"different product id", base + s.MainImagePrefix(5, 99) + "photo.jpg"},
		{"different host", "http://evil.example.com/" + testPublicContainer + "/" + s.MainImagePrefix(5, 12) + "photo.jpg"},
		{"private container instead of public", endpoint + "/" + testPrivateContainer + "/" + s.MainImagePrefix(5, 12) + "photo.jpg"},
		{"invalid url", "://not-a-url"},
		{"just the prefix, no filename", base + s.MainImagePrefix(5, 12)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.ImageBlobNameFromURL(tc.url, 5, 12); err == nil {
				t.Errorf("ImageBlobNameFromURL(%q) expected error, got nil", tc.url)
			}
		})
	}
}

func TestFileBlobNameFromURL(t *testing.T) {
	s := newTestStorage(t)
	endpoint := strings.TrimSuffix(s.BlobEndpoint(), "/")

	validURL := endpoint + "/" + testPrivateContainer + "/" + s.ProductFilePrefix(5, 12) + "ebook.pdf"
	got, err := s.FileBlobNameFromURL(validURL, 5, 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "develop/products/5/12/product_file/ebook.pdf"; got != want {
		t.Errorf("FileBlobNameFromURL() = %q, want %q", got, want)
	}

	// 公開コンテナ側のURLをfile用として渡した場合は拒否される。
	publicURL := endpoint + "/" + testPublicContainer + "/" + s.ProductFilePrefix(5, 12) + "ebook.pdf"
	if _, err := s.FileBlobNameFromURL(publicURL, 5, 12); err == nil {
		t.Errorf("FileBlobNameFromURL(%q) expected error, got nil", publicURL)
	}
}

func TestClassifyImageBlobName_Unknown(t *testing.T) {
	s := newTestStorage(t)
	blobName := s.ProductPathPrefix(5, 12) + "other/photo.jpg"
	if kind := s.ClassifyImageBlobName(blobName, 5, 12); kind != ImageKindUnknown {
		t.Errorf("ClassifyImageBlobName() = %v, want ImageKindUnknown", kind)
	}
}
