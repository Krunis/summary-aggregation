package aggregator

import (
	"context"
	"net"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"google.golang.org/grpc"
)

type AggregatorService struct {
	port string

	grpcServer *grpc.Server
	lis net.Listener

	DBPool *pgxpool.Pool

	RedisDB *redis.Conn

	Lifecycle struct {
		Ctx    context.Context
		Cancel context.CancelFunc
	}

	wg sync.WaitGroup

	stopOnce sync.Once
}

