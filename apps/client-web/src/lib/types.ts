export type Product = {
  id: number;
  userId: number;
  categoryId: number;
  name: string;
  description: string;
  imageUrl: string;
  price: number;
  status: string;
  createdAt: string;
  updatedAt: string;
};

export type CartItem = Pick<Product, "id" | "name" | "price" | "imageUrl"> & { quantity: number };
