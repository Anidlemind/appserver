package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "carstats/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	addr := envOrDefault("GRPC_DEMO_ADDR", "127.0.0.1:50051")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("dial grpc server: %v", err)
	}
	defer conn.Close()

	authClient := pb.NewAuthClient(conn)
	statsClient := pb.NewStatsServiceClient(conn)

	suffix := time.Now().UTC().Format("20060102150405.000000000")
	email := "demo-" + strings.ReplaceAll(suffix, ".", "") + "@example.com"
	password := "secret123"
	vehicleType := "sedan-demo-" + strings.ReplaceAll(suffix, ".", "")

	fmt.Println("Register request:")
	fmt.Printf("{email:%q, password:%q}\n", email, password)

	registerResp, err := authClient.Register(ctx, &pb.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		log.Fatalf("register: %v", err)
	}

	fmt.Println("Register response:")
	fmt.Printf("{user_id:%d, token:%q}\n\n", registerResp.GetUserId(), registerResp.GetToken())

	fmt.Println("Login request:")
	fmt.Printf("{email:%q, password:%q}\n", email, password)

	loginResp, err := authClient.Login(ctx, &pb.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		log.Fatalf("login: %v", err)
	}

	fmt.Println("Login response:")
	fmt.Printf("{token:%q}\n\n", loginResp.GetToken())

	authCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"authorization", "Bearer "+loginResp.GetToken(),
	))

	firstStat := &pb.SubmitUserStatsRequest{
		VehicleType: vehicleType,
		FuelLiters:  20.5,
		DistanceKm:  250,
		RecordedAt:  "2026-04-21T10:30:00Z",
	}

	secondStat := &pb.SubmitUserStatsRequest{
		VehicleType: vehicleType,
		FuelLiters:  10,
		DistanceKm:  100,
		RecordedAt:  "2026-04-21T10:35:00Z",
	}

	fmt.Println("SubmitUserStats request #1:")
	fmt.Printf("{vehicle_type:%q, fuel_liters:%v, distance_km:%v, recorded_at:%q}\n", firstStat.GetVehicleType(), firstStat.GetFuelLiters(), firstStat.GetDistanceKm(), firstStat.GetRecordedAt())
	submitResp, err := statsClient.SubmitUserStats(authCtx, firstStat)
	if err != nil {
		log.Fatalf("submit first stat: %v", err)
	}
	fmt.Println("SubmitUserStats response #1:")
	fmt.Printf("{success:%t, message:%q}\n\n", submitResp.GetSuccess(), submitResp.GetMessage())

	fmt.Println("SubmitUserStats request #2:")
	fmt.Printf("{vehicle_type:%q, fuel_liters:%v, distance_km:%v, recorded_at:%q}\n", secondStat.GetVehicleType(), secondStat.GetFuelLiters(), secondStat.GetDistanceKm(), secondStat.GetRecordedAt())
	submitResp, err = statsClient.SubmitUserStats(authCtx, secondStat)
	if err != nil {
		log.Fatalf("submit second stat: %v", err)
	}
	fmt.Println("SubmitUserStats response #2:")
	fmt.Printf("{success:%t, message:%q}\n\n", submitResp.GetSuccess(), submitResp.GetMessage())

	fmt.Println("GetGlobalStats request:")
	fmt.Printf("{vehicle_type:%q}\n", vehicleType)

	statsResp, err := statsClient.GetGlobalStats(authCtx, &pb.GetGlobalStatsRequest{
		VehicleType: vehicleType,
	})
	if err != nil {
		log.Fatalf("get global stats: %v", err)
	}

	fmt.Println("GetGlobalStats response:")
	fmt.Printf("{vehicle_type:%q, total_fuel_liters:%v, total_distance_km:%v, total_records:%d, avg_fuel_per_km:%v}\n",
		statsResp.GetVehicleType(),
		statsResp.GetTotalFuelLiters(),
		statsResp.GetTotalDistanceKm(),
		statsResp.GetTotalRecords(),
		statsResp.GetAvgFuelPerKm(),
	)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
