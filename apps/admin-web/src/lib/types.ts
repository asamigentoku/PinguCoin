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

export type User = {
  id: number;
  email: string;
  name: string;
  createdAt: string;
  updatedAt: string;
};
