package aggregator

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type SummaryRepository interface {
	GetField(context.Context, string, string) (string, error)
	Close()
}

type PostgresSummaryRepository struct{
	DBPool *pgxpool.Pool
}

func (sr *PostgresSummaryRepository) Close(){
	sr.DBPool.Close()
}
