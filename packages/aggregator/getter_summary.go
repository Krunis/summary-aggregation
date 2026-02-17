package aggregator

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/Krunis/summary-aggregation/packages/common"
	pb "github.com/Krunis/summary-aggregation/packages/grpcapi"
	"github.com/jackc/pgx/v4"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (sr *PostgresSummaryRepository) GetField(ctx context.Context, username, field string) (string, error) {
	row := sr.DBPool.QueryRow(ctx, `SELECT $1
									FROM summary
									WHERE username = $2`, field, username)

	var value string

	if err := row.Scan(&value); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	return value, nil
}

func (as *AggregatorService) GetFromCache(ctx context.Context, key string) (string, error) {
	id, err := as.RedisDB.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", errors.New("no key in redis")
		} else {
			return "", err
		}
	}

	return id, nil
}

func (as *AggregatorService) GetFromDB(ctx context.Context, username string) (*pb.DBSummary, error) {
	keyID := common.SummaryUserExampleKey + username + common.SummaryIDPrefix

	id, err := as.GetFromCache(ctx, keyID)
	if err != nil {
		log.Printf("Failed to get ID from cache: %s", err)

		id, err = as.DBRepo.GetField(ctx, username, "id")
		if err != nil {
			log.Printf("Failed to get ID from postgres: %s", err)
		}

		go func() {
			ctxSend, cancel := context.WithTimeout(context.Background(), time.Millisecond*250)
			defer cancel()

			if err := as.SendInCache(ctxSend, keyID, id); err != nil {
				log.Printf("Failed to send ID in cache: %s", err)
			}
		}()
	}

	keyLightType := common.SummaryUserExampleKey + username + common.SummaryLightTypePrefix

	lightType, err := as.GetFromCache(ctx, keyLightType)
	if err != nil {
		log.Printf("Failed to get light type from cache: %s", err)

		id, err = as.DBRepo.GetField(ctx, username, "light_type")
		if err != nil {
			log.Printf("Failed to get light type from postgres: %s", err)
		}

		go func() {
			ctxSend, cancel := context.WithTimeout(context.Background(), time.Millisecond*250)
			defer cancel()

			if err := as.SendInCache(ctxSend, keyLightType, lightType); err != nil {
				log.Printf("Failed to send light type in cache: %s", err)
			}
		}()
	}

	return &pb.DBSummary{Id: id, LightType: lightType}, nil

}

func (as *AggregatorService) SendInCache(ctx context.Context, key, value string) error {
	_, err := as.RedisDB.Set(ctx, key, value, time.Second*5).Result()
	if err != nil {
		return err
	}

	return nil
}

func (as *AggregatorService) GetFromService(ctx context.Context, username string) (*pb.ServiceSummary, error) {
	time.Sleep(time.Millisecond * 200)

	return &pb.ServiceSummary{Time: timestamppb.Now(), Color: "red"}, nil
}
