package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/shadyar-bakr/greenlight/internal/data"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	limiter struct {
		rps     float64
		burst   int
		cleanup time.Duration
		enabled bool
	}
}

type application struct {
	config config
	logger *slog.Logger
	models data.Models
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		os.Exit(1)
	}

	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max idle time")
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.DurationVar(&cfg.limiter.cleanup, "limiter-cleanup", 3*time.Minute, "Rate limiter cleanup time")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", false, "Enable rate limiter")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	db, err := openDB(cfg, logger)
	if err != nil {
		logger.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close(context.Background())

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
	}

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func openDB(cfg config, logger *slog.Logger) (*pgx.Conn, error) {
	connConfig, err := pgx.ParseConfig(cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	conn, err := pgx.ConnectConfig(context.Background(), connConfig)
	if err != nil {
		return nil, err
	}

	err = conn.Ping(context.Background())
	if err != nil {
		conn.Close(context.Background())
		return nil, err
	}

	logger.Info("db connected", "dsn", cfg.db.dsn)
	return conn, nil
}
