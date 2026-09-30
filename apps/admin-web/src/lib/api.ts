import type { Product, User } from "./types";

const endpoint = process.env.PINGU_API_URL ?? "http://localhost:8082/api/v1/graphql";
type GraphQLResponse<T> = { data?: T; errors?: Array<{ message: string }> };

async function graphql<T>(query: string, variables?: Record<string, unknown>) {
  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "content-type": "application/json" },
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
const userFields = `id email name createdAt updatedAt`;

export async function getProducts(): Promise<Product[]> {
  const data = await graphql<{ products: Product[] }>(`query AdminProducts { products { ${productFields} } }`);
  return data.products;
}

export async function getUsers(): Promise<User[]> {
  const data = await graphql<{ users: User[] }>(`query AdminUsers { users { ${userFields} } }`);
  return data.users;
}
