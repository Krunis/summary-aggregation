package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
)

// ---- типы ----
type AggregationService struct {
	redisDB *redis.Client
	pgDB    *sql.DB
}

type AggregatedData struct {
	ListA []string `json:"list_a,omitempty"`
	ListB []string `json:"list_b,omitempty"`
}

// ---- методы ----

// Получаем данные из Redis с таймаутом
func (s *AggregationService) GetFromCache(ctx context.Context, key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	result, err := s.redisDB.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var data []string
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, err
	}
	return data, nil
}

// Получаем данные из Postgres с таймаутом
func (s *AggregationService) GetFromPostgres(ctx context.Context, query string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	rows, err := s.pgDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

// Основной handler — агрегируем данные из нескольких источников
func (s *AggregationService) AggregateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	agg := AggregatedData{}

	// --- Получаем ListA ---
	listA, err := s.GetFromCache(ctx, "list_a")
	if err != nil {
		log.Printf("Cache error: %v", err)
	}

	if listA == nil {
		listA, err = s.GetFromPostgres(ctx, `SELECT content FROM lists WHERE list_name='A'`)
		if err != nil {
			log.Printf("Postgres error ListA: %v", err)
		}
		// fallback: если удалось получить из Postgres, сохраняем в Redis
		if listA != nil {
			dataJSON, _ := json.Marshal(listA)
			s.redisDB.Set(ctx, "list_a", dataJSON, time.Minute)
		}
	}
	agg.ListA = listA

	// --- Получаем ListB ---
	listB, err := s.GetFromCache(ctx, "list_b")
	if err != nil {
		log.Printf("Cache error: %v", err)
	}

	if listB == nil {
		listB, err = s.GetFromPostgres(ctx, `SELECT content FROM lists WHERE list_name='B'`)
		if err != nil {
			log.Printf("Postgres error ListB: %v", err)
		}
		if listB != nil {
			dataJSON, _ := json.Marshal(listB)
			s.redisDB.Set(ctx, "list_b", dataJSON, time.Minute)
		}
	}
	agg.ListB = listB

	// --- Частичное восстановление данных ---
	if agg.ListA == nil && agg.ListB == nil {
		http.Error(w, "no data available", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agg)
}

// ---- main ----
func main() {
	// Подключение к Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Подключение к Postgres
	pgDB, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/dbname?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	service := &AggregationService{
		redisDB: rdb,
		pgDB:    pgDB,
	}

	http.HandleFunc("/aggregate", service.AggregateHandler)
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}