package aggregator

import (
	"context"
	"log"
	"time"

	"github.com/Krunis/summary-aggregation/packages/common"
	"github.com/redis/go-redis/v9"
)

type DBSummary struct {
	id         string
	light_type string
}

type ServiceSummary struct {
	date  time.Time
	color string
}

func (as *AggregatorService) GetFromPostgres(ctx context.Context, username string) (*DBSummary, error) {
	summ := &DBSummary{}

	row := as.DBPool.QueryRow(ctx, `SELECT id, light_type
									FROM summary
									WHERE username = $1`, username)
	
	if err := row.Scan(&summ.id, &summ.light_type); err != nil{
		return nil, err
	}

	return summ, nil
}

func (as *AggregatorService) GetFromRedis(ctx context.Context, username string) (*DBSummary, error) {
	keyID := common.SummaryUserExampleKey + username + common.SummaryIDPrefix

	id, err := as.RedisDB.Get(ctx, keyID).Result()
	if err != nil {
		if err == redis.Nil {
			log.Println("No cache id")
		} else {
			log.Printf("Failed to get ID from Redis: %s", err)
			return nil, err
		}
	}

	keyLightType := common.SummaryUserExampleKey + username + common.SummaryLightTypePrefix

	lightType, err := as.RedisDB.Get(ctx, keyLightType).Result()
	if err != nil {
		if err == redis.Nil {
			log.Println("No cache light type")
		} else {
			log.Printf("Failed to get light type from Redis: %s", err)
			return nil, err
		}
	}

	return &DBSummary{id: id, light_type: lightType}, nil

}

func (as *AggregatorService) GetFromService() (*ServiceSummary, error) {
	time.Sleep(time.Millisecond * 500)

	return &ServiceSummary{date: time.Now(), color: "red"}, nil
}
