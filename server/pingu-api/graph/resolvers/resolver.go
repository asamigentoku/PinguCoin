package resolvers

import (
	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/repository"
)

// Resolver holds the dependencies shared by all GraphQL resolvers.
type Resolver struct {
	Orcan  *orcanclient.Client
	Orders *repository.OrderRepository
}

var _ graph.ResolverRoot = (*Resolver)(nil)

func (r *Resolver) Mutation() graph.MutationResolver { return &mutationResolver{Resolver: r} }
func (r *Resolver) Query() graph.QueryResolver       { return &queryResolver{Resolver: r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
