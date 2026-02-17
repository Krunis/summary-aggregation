package aggreagationserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	pb "github.com/Krunis/summary-aggregation/packages/grpcapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AggregationServerService struct {
	port string

	httpServer *http.Server
	mux        *http.ServeMux

	aggregatorAddress string
	grpcConn          *grpc.ClientConn
	grpcClient        pb.AggregatorServiceClient

	wg sync.WaitGroup

	Lifecycle struct {
		Ctx    context.Context
		Cancel context.CancelFunc
	}

	stopOnce sync.Once
}

func NewAggregationServerService(serverPort string) *AggregationServerService {
	mux := http.NewServeMux()

	ctx, cancel := context.WithCancel(context.Background())

	return &AggregationServerService{
		port: serverPort,
		mux:  mux,
		Lifecycle: struct {
			Ctx    context.Context
			Cancel context.CancelFunc
		}{
			Ctx:    ctx,
			Cancel: cancel},
	}
}

func (as *AggregationServerService) Start(aggregatorAddress string) error {
	as.aggregatorAddress = aggregatorAddress

	if err := as.connectToAggregator(); err != nil {
		return err
	}

	as.mux.HandleFunc("/user-summary", as.UserSummaryHandler)
	as.mux.HandleFunc("/aggregation-health", as.AggregationHealthHandler)

	as.httpServer = &http.Server{}

	as.httpServer.Addr = as.port
	as.httpServer.Handler = as.mux

	if err := as.RunHTTPServer(); err != nil {
		log.Printf("Stop running error: %s", err)
	}

	return as.Stop()
}

func (as *AggregationServerService) RunHTTPServer() error {
	errCh := make(chan error, 1)

	stopCh := make(chan os.Signal, 1)

	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopCh)

	log.Printf("Starting listening on %s...\n", as.port)
	go func() {
		if err := as.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		} else {
			errCh <- nil
		}
	}()

	select {
	case <-stopCh:
		return fmt.Errorf("received OS signal")
	case err := <-errCh:
		return err
	case <-as.Lifecycle.Ctx.Done():
		return nil
	}
}

func (as *AggregationServerService) connectToAggregator() error {
	var err error

	log.Printf("Connecting to %s...", as.aggregatorAddress)

	as.grpcConn, err = grpc.NewClient(as.aggregatorAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	as.grpcClient = pb.NewAggregatorServiceClient(as.grpcConn)

	return nil
}

func (as *AggregationServerService) UserSummaryHandler(w http.ResponseWriter, r *http.Request) {
	select{
	case <-as.Lifecycle.Ctx.Done():
		log.Println("Request cancelled (shutdown or client disconnected)")
		return
	default:
		if r.Method != "POST"{
			http.Error(w, "Only POST method is allowed", http.StatusBadRequest)
			return
		}

		summReq := &pb.UserSummaryRequest{}

		err := json.NewDecoder(r.Body).Decode(&summReq)
		if err != nil{
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("Received request: %v\n", summReq)

		if err := ValidateSummReq(summReq); err != nil{
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Millisecond * 300)
		defer cancel()

		resp, err := as.grpcClient.GetUserSummary(ctx, summReq)
		if err != nil{
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if resp == nil{
			http.Error(w, "undefined", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		err = json.NewEncoder(w).Encode(resp)
		if err != nil{
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}


	}
}

func (as *AggregationServerService) AggregationHealthHandler(w http.ResponseWriter, r *http.Request) {
	return
}

func (as *AggregationServerService) Stop() error {
	var result error

	as.stopOnce.Do(func() {
		var errs []error

		as.Lifecycle.Cancel()

		as.wg.Wait()

		func() {
			if as.httpServer != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*15)
				defer cancel()

				log.Println("Graceful shutdown...")
				if err := as.httpServer.Shutdown(shutdownCtx); err != nil {
					err = fmt.Errorf("graceful shutdown failed: %s\n", err)
					errs = append(errs, err)

					if err = as.httpServer.Close(); err != nil {
						log.Printf("Force close failed: %s\n", err)
						err = fmt.Errorf("shutdown failed: %v, close failed: %v", err, err)
						errs = append(errs, err)
					}
					err = fmt.Errorf("shutdown failed: %v, forced close", err)
				}
			}
		}()

		if as.grpcConn != nil {
			if err := as.grpcConn.Close(); err != nil {
				
			}
		}
		if len(errs) > 0 {
			result = errors.Join(errs...)
		}

		log.Println("Shutdown completed")

	})

	return result
}
