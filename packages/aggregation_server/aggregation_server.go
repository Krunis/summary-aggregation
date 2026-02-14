package aggreagationserver

import (
	"context"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
)

type AggregationServerService struct {
	port       string
	httpServer *http.Server
	mux        *http.ServeMux

	DBPool *pgxpool.Pool

	Lifecycle struct {
		Ctx    context.Context
		Cancel context.CancelFunc
	}

	wg sync.WaitGroup

	stopOnce sync.Once
}