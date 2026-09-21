package grpcserver

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
)

// UserServer は proto の `service UserService` の実装。
// 認証はClerk(フロントエンド)が担うため、ここでは認証情報を一切扱わない。
type UserServer struct {
	pb.UnimplementedUserServiceServer
	repo *repository.UserRepository
}

func NewUserServer(repo *repository.UserRepository) *UserServer {
	return &UserServer{repo: repo}
}

func (server *UserServer) ListUsers(ctx context.Context, request *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	users, err := server.repo.FindAll()
	if err != nil {
		return nil, apperr.Internal(err)
	}

	response := &pb.ListUsersResponse{}
	for i := range users {
		response.Users = append(response.Users, toProtoUser(&users[i]))
	}
	return response, nil
}

func (server *UserServer) GetUser(ctx context.Context, request *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("user", err)
	}
	return &pb.GetUserResponse{User: toProtoUser(user)}, nil
}

func (server *UserServer) GetUserByClerkID(ctx context.Context, request *pb.GetUserByClerkIDRequest) (*pb.GetUserByClerkIDResponse, error) {
	clerkUserID := strings.TrimSpace(request.GetClerkUserId())
	if clerkUserID == "" {
		return nil, apperr.InvalidArgument("clerk_user_id is required")
	}

	user, err := server.repo.FindByClerkUserID(clerkUserID)
	if err != nil {
		return nil, mapFindError("user", err)
	}
	return &pb.GetUserByClerkIDResponse{User: toProtoUser(user)}, nil
}

// EnsureUser はclerk_user_idに対応するUserを取得し、無ければ作成する(pingu-apiが
// Clerkトークン検証後の初回アクセス時に呼ぶJITプロビジョニング用)。
// 既に存在する場合、email/nameは上書きしない(プロフィール更新はUpdateUserで行う)。
func (server *UserServer) EnsureUser(ctx context.Context, request *pb.EnsureUserRequest) (*pb.EnsureUserResponse, error) {
	clerkUserID := strings.TrimSpace(request.GetClerkUserId())
	if clerkUserID == "" {
		return nil, apperr.InvalidArgument("clerk_user_id is required")
	}

	user, err := server.repo.FindByClerkUserID(clerkUserID)
	if err == nil {
		return &pb.EnsureUserResponse{User: toProtoUser(user)}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.Internal(err)
	}

	user = &model.User{
		ClerkUserID: clerkUserID,
		Email:       strings.TrimSpace(request.GetEmail()),
		Name:        strings.TrimSpace(request.GetName()),
	}
	if err := server.repo.Create(user); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.EnsureUserResponse{User: toProtoUser(user)}, nil
}

// UpdateUser は氏名のみを更新する(emailはClerk側が真実の記録を持つため今回のスコープ外)。
func (server *UserServer) UpdateUser(ctx context.Context, request *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return nil, apperr.InvalidArgument("name is required")
	}

	user, err := server.repo.FindByID(uint(request.GetId()))
	if err != nil {
		return nil, mapFindError("user", err)
	}

	user.Name = name
	if err := server.repo.Update(user); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateUserResponse{User: toProtoUser(user)}, nil
}

func (server *UserServer) DeleteUser(ctx context.Context, request *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	if err := server.repo.Delete(uint(request.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteUserResponse{}, nil
}

func toProtoUser(user *model.User) *pb.User {
	return &pb.User{
		Id:          uint32(user.ID),
		ClerkUserId: user.ClerkUserID,
		Email:       user.Email,
		Name:        user.Name,
		CreatedAt:   timestamppb.New(user.CreatedAt),
		UpdatedAt:   timestamppb.New(user.UpdatedAt),
	}
}
