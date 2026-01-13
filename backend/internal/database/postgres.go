package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/ai-budget-planner/backend/internal/config"
)

// Open открывает пул подключений к PostgreSQL с ретраями.
func Open(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolConfig, cfgErr := pgxpool.ParseConfig(cfg.DSN())
	if cfgErr != nil {
		return nil, fmt.Errorf("parse database config: %w", cfgErr)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	// MaxIdleConns maps closest to MinConns in pgxpool.
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime

	var pool *pgxpool.Pool
	var err error

	retries := 5
	backoff := time.Second * 1

	for i := 0; i < retries; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = pool.Ping(pingCtx)
			cancel()

			if err == nil {
				return pool, nil // Успех! Возвращаем пул
			}
		}

		if pool != nil {
			pool.Close() // Закрываем старый пул перед ретраем
		}

		log.Printf("Попытка подключения %d/%d не удалась: %v. Повтор через %v", i+1, retries, err, backoff)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	return nil, fmt.Errorf("не удалось подключиться к БД после %d попыток: %w", retries, err)
}
