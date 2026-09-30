import type { Product } from "./types";

const endpoint = process.env.PINGU_API_URL ?? "http://localhost:8082/api/v1/graphql";
type GraphQLResponse<T> = { data?: T; errors?: Array<{ message: string }> };

export async function graphql<T>(query: string, variables?: Record<string, unknown>, token?: string) {
  const response = await fetch(endpoint, {
    method: "POST",
    headers: token ? { "content-type": "application/json", authorization: `Bearer ${token}` } : { "content-type": "application/json" },
    body: JSON.stringify({ query, variables }),
    cache: "no-store",
    signal: AbortSignal.timeout(5000),
  });
  if (!response.ok) throw new Error(`Pingu API returned ${response.status}`);
  const payload = (await response.json()) as GraphQLResponse<T>;
  if (payload.errors?.length || !payload.data) throw new Error(payload.errors?.[0]?.message ?? "Invalid GraphQL response");
  return payload.data;
}

const productFields = `id userId categoryId name description imageUrl price status createdAt updatedAt`;

export async function getProducts(): Promise<Product[]> {
  const data = await graphql<{ products: Product[] }>(`query StorefrontProducts { products { ${productFields} } }`);
  // 下書きは出品者本人の管理ページにだけ表示する。
  return data.products.filter((product) => product.status !== "draft");
}

export async function getProduct(id: number): Promise<Product | null> {
  const data = await graphql<{ product: Product | null }>(`query StorefrontProduct($id: Int!) { product(id: $id) { ${productFields} } }`, { id });
  return data.product;
}
