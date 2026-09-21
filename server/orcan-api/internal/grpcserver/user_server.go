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

func (s *UserServer) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	users, err := s.repo.FindAll()
	if err != nil {
		return nil, apperr.Internal(err)
	}

	resp := &pb.ListUsersResponse{}
	for i := range users {
		resp.Users = append(resp.Users, toProtoUser(&users[i]))
	}
	return resp, nil
}

func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("user", err)
	}
	return &pb.GetUserResponse{User: toProtoUser(user)}, nil
}

func (s *UserServer) GetUserByClerkID(ctx context.Context, req *pb.GetUserByClerkIDRequest) (*pb.GetUserByClerkIDResponse, error) {
	clerkUserID := strings.TrimSpace(req.GetClerkUserId())
	if clerkUserID == "" {
		return nil, apperr.InvalidArgument("clerk_user_id is required")
	}

	user, err := s.repo.FindByClerkUserID(clerkUserID)
	if err != nil {
		return nil, mapFindError("user", err)
	}
	return &pb.GetUserByClerkIDResponse{User: toProtoUser(user)}, nil
}

// EnsureUser はclerk_user_idに対応するUserを取得し、無ければ作成する(pingu-apiが
// Clerkトークン検証後の初回アクセス時に呼ぶJITプロビジョニング用)。
// 既に存在する場合、email/nameは上書きしない(プロフィール更新はUpdateUserで行う)。
func (s *UserServer) EnsureUser(ctx context.Context, req *pb.EnsureUserRequest) (*pb.EnsureUserResponse, error) {
	clerkUserID := strings.TrimSpace(req.GetClerkUserId())
	if clerkUserID == "" {
		return nil, apperr.InvalidArgument("clerk_user_id is required")
	}

	user, err := s.repo.FindByClerkUserID(clerkUserID)
	if err == nil {
		return &pb.EnsureUserResponse{User: toProtoUser(user)}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.Internal(err)
	}

	user = &model.User{
		ClerkUserID: clerkUserID,
		Email:       strings.TrimSpace(req.GetEmail()),
		Name:        strings.TrimSpace(req.GetName()),
	}
	if err := s.repo.Create(user); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.EnsureUserResponse{User: toProtoUser(user)}, nil
}

// UpdateUser は氏名のみを更新する(emailはClerk側が真実の記録を持つため今回のスコープ外)。
func (s *UserServer) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, apperr.InvalidArgument("name is required")
	}

	user, err := s.repo.FindByID(uint(req.GetId()))
	if err != nil {
		return nil, mapFindError("user", err)
	}

	user.Name = name
	if err := s.repo.Update(user); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.UpdateUserResponse{User: toProtoUser(user)}, nil
}

func (s *UserServer) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	if err := s.repo.Delete(uint(req.GetId())); err != nil {
		return nil, apperr.Internal(err)
	}
	return &pb.DeleteUserResponse{}, nil
}

func toProtoUser(u *model.User) *pb.User {
	return &pb.User{
		Id:          uint32(u.ID),
		ClerkUserId: u.ClerkUserID,
		Email:       u.Email,
		Name:        u.Name,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		UpdatedAt:   timestamppb.New(u.UpdatedAt),
	}
}
