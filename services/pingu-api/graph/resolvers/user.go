package resolvers

import (
	"context"

	orcanpb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/graph/converters"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/graph/model"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/reqcontext"
)

func (r *mutationResolver) UpdateUser(ctx context.Context, id int32, input model.UpdateUserInput) (*model.User, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	if claims.UserID != uint(id) {
		return nil, apperr.Unauthenticated("cannot update another user")
	}
	response, err := r.Orcan.User.UpdateUser(ctx, &orcanpb.UpdateUserRequest{Id: uint32(id), Name: input.Name})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.UserFromPB(response.GetUser()), nil
}

func (r *queryResolver) Users(ctx context.Context) ([]*model.User, error) {
	response, err := r.Orcan.User.ListUsers(ctx, &orcanpb.ListUsersRequest{})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	users := make([]*model.User, 0, len(response.GetUsers()))
	for _, user := range response.GetUsers() {
		users = append(users, converters.UserFromPB(user))
	}
	return users, nil
}

func (r *queryResolver) User(ctx context.Context, id int32) (*model.User, error) {
	response, err := r.Orcan.User.GetUser(ctx, &orcanpb.GetUserRequest{Id: uint32(id)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.UserFromPB(response.GetUser()), nil
}

func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	claims, ok := reqcontext.UserFromContext(ctx)
	if !ok {
		return nil, apperr.Unauthenticated("login is required")
	}
	response, err := r.Orcan.User.GetUser(ctx, &orcanpb.GetUserRequest{Id: uint32(claims.UserID)})
	if err != nil {
		return nil, apperr.FromGRPC(err)
	}
	return converters.UserFromPB(response.GetUser()), nil
}
