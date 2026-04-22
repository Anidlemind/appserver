package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"carstats/internal/aggregation"
	"carstats/internal/db"
	"carstats/internal/httpgateway"
	"carstats/internal/service"
	"carstats/internal/sso"
	pb "carstats/proto"

	"github.com/robfig/cron/v3"
	"google.golang.org/grpc"
)

func main() {

	pool, err := db.NewPool()
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	tokenTTL := readDurationEnv("AUTH_TOKEN_TTL", 24*time.Hour)
	aggregationSchedule := readStringEnv("AGGREGATION_SCHEDULE", "@every 5m")
	grpcAddr := readStringEnv("GRPC_ADDR", ":50051")
	httpAddr := readStringEnv("HTTP_ADDR", ":8080")

	authService := sso.New(pool, tokenTTL)
	statsService := service.NewStatsServer(pool)
	aggregator := aggregation.NewAggregator(pool)

	c := cron.New()
	if _, err := c.AddFunc(aggregationSchedule, aggregator.Run); err != nil {
		log.Fatal(err)
	}
	c.Start()
	defer c.Stop()

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(sso.UnaryAuthInterceptor(authService)),
	)

	pb.RegisterAuthServer(grpcServer, authService)
	pb.RegisterStatsServiceServer(grpcServer, statsService)

	gateway, err := httpgateway.New(context.Background(), grpcAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer gateway.Close()

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: gateway.Handler(),
	}

	go func() {
		log.Printf("http gateway started on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	log.Printf("grpc server started on %s", grpcAddr)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

func readStringEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func readDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("invalid %s=%q, fallback to %s", key, value, fallback)
		return fallback
	}

	return parsed
}
