package aggregator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Krunis/summary-aggregation/packages/common"
	pb "github.com/Krunis/summary-aggregation/packages/grpcapi"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type AggregatorService struct {
	pb.UnimplementedAggregatorServiceServer

	port string

	grpcServer *grpc.Server
	lis        net.Listener

	DBRepo SummaryRepository

	RedisDB *redis.Client

	Lifecycle struct {
		Ctx    context.Context
		Cancel context.CancelFunc
	}

	wg sync.WaitGroup

	stopOnce sync.Once
}

func NewAggregatorService(aggregatorPort string) *AggregatorService {
	ctx, cancel := context.WithCancel(context.Background())

	return &AggregatorService{
		port: aggregatorPort,
		Lifecycle: struct {
			Ctx    context.Context
			Cancel context.CancelFunc
		}{Ctx: ctx, Cancel: cancel}}
}

func (as *AggregatorService) Start(postgresConnectionString string) error {
	var err error

	pool, err := common.ConnectToPostgres(as.Lifecycle.Ctx, postgresConnectionString)
	if err != nil {
		return err
	}
	
	as.DBRepo = &PostgresSummaryRepository{DBPool: pool}

	as.RedisDB, err = common.ConnectToRedis(as.Lifecycle.Ctx)
	if err != nil {
		return err
	}

	if err := as.RunServer(); err != nil {
		log.Printf("Stop running error: %s", err)
	}

	return as.Stop()
}

func (as *AggregatorService) RunServer() error {
	errCh := make(chan error, 1)

	stopCh := make(chan os.Signal, 1)

	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopCh)

	go func() {
		if err := as.startGRPCServer(); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-stopCh:
		return fmt.Errorf("received OS signal")
	case <-as.Lifecycle.Ctx.Done():
		return as.Lifecycle.Ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (as *AggregatorService) startGRPCServer() error {
	var err error

	as.lis, err = net.Listen("tcp", as.port)
	if err != nil {
		log.Printf("Failed to start listening on %s", as.port)
		return err
	}

	log.Printf("Starting listening on %s...", as.port)

	as.grpcServer = grpc.NewServer()

	pb.RegisterAggregatorServiceServer(as.grpcServer, as)

	if err := as.grpcServer.Serve(as.lis); err != nil {
		return err
	}

	return nil
}

func (as *AggregatorService) GetUserSummary(ctx context.Context, req *pb.UserSummaryRequest) (*pb.UserSummaryResponse, error) {
	select {
	case <-as.Lifecycle.Ctx.Done():
		return nil, as.Lifecycle.Ctx.Err()
	default:
		var err error

		resp := &pb.UserSummaryResponse{}

		usersumm := req.GetUserSummaryName()
		resp.UserSummaryName = usersumm

		resp.DbSummary, err = as.GetFromDB(ctx, usersumm)
		if err != nil{
			log.Printf("Failed to get from DB: %s", err)
		}

		resp.ServiceSummmary, err = as.GetFromService(ctx, usersumm)
		if err != nil{
			log.Printf("Failed to get from service: %s", err)
		}

		return resp, nil
	}
}

func (as *AggregatorService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error){
	log.Printf("%s doing health check...", req.GetUsername())

	resp := &pb.HealthCheckResponse{
		HealthDb: &pb.HealthDB{},
	}

	ctxRedis, cancel := context.WithTimeout(ctx, time.Millisecond * 500)
	defer cancel()

	err := as.RedisDB.Ping(ctxRedis).Err()
	if err != nil{
		log.Printf("Health redis error: %s", err)
		resp.HealthDb.RedisHealth = false
	}else{
		resp.HealthDb.RedisHealth = true
	}

	ctxPostgres, cancel := context.WithTimeout(ctx, time.Millisecond * 500)
	defer cancel()

	err = as.DBRepo.healthLook(ctxPostgres)
	if err != nil{
		log.Printf("Health postgres error: %s", err)
		resp.HealthDb.PostgresHealth = false
	}else{
		resp.HealthDb.PostgresHealth = true
	}

	if true{
		resp.HealthOtherService = true
	}

	return resp, nil
}

func (as *AggregatorService) Stop() error {
	var result error

	as.stopOnce.Do(func() {
		var errs []error

		as.Lifecycle.Cancel()

		as.wg.Wait()

		log.Println("Closing listener...")
		if as.lis != nil {
			if err := as.lis.Close(); err != nil {
				errs = append(errs, err)
			}
		}

		log.Println("Closing GRPC server...")
		if as.grpcServer != nil {
			as.grpcServer.GracefulStop()
		}

		if as.DBRepo != nil {
			as.DBRepo.Close()
		}

		if as.RedisDB != nil {
			if err := as.RedisDB.Close(); err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			result = errors.Join(errs...)
		}
	})

	return result
}
