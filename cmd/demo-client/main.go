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
	vehicleType := "sedan"
	brand := "Kia"
	model := "Rio-" + strings.ReplaceAll(suffix, ".", "")

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
		VehicleId:                     "demo-vehicle-1",
		Brand:                         brand,
		Model:                         model,
		VehicleType:                   vehicleType,
		ManualAvgConsumptionLPer_100Km: 8.2,
		ObdAvgConsumptionLPer_100Km:    8.6,
		AvgRefuelLiters:               34.1,
		AvgDistanceBetweenRefuelsKm:   410,
		ManualRecordsCount:            5,
		TelemetrySamplesCount:         1861,
		UpdatedAt:                     "2026-04-24T19:00:00Z",
	}

	secondStat := &pb.SubmitUserStatsRequest{
		VehicleId:                     "demo-vehicle-2",
		Brand:                         brand,
		Model:                         model,
		VehicleType:                   vehicleType,
		ManualAvgConsumptionLPer_100Km: 8.4,
		ObdAvgConsumptionLPer_100Km:    8.7,
		AvgRefuelLiters:               35.0,
		AvgDistanceBetweenRefuelsKm:   420,
		ManualRecordsCount:            6,
		TelemetrySamplesCount:         2000,
		UpdatedAt:                     "2026-04-24T20:00:00Z",
	}

	fmt.Println("SubmitUserStats request #1:")
	fmt.Printf("{vehicle_id:%q, brand:%q, model:%q, vehicle_type:%q, manual_avg_consumption_l_per_100km:%v, obd_avg_consumption_l_per_100km:%v, avg_refuel_liters:%v, avg_distance_between_refuels_km:%v, manual_records_count:%d, telemetry_samples_count:%d, updated_at:%q}\n",
		firstStat.GetVehicleId(), firstStat.GetBrand(), firstStat.GetModel(), firstStat.GetVehicleType(),
		firstStat.GetManualAvgConsumptionLPer_100Km(), firstStat.GetObdAvgConsumptionLPer_100Km(),
		firstStat.GetAvgRefuelLiters(), firstStat.GetAvgDistanceBetweenRefuelsKm(),
		firstStat.GetManualRecordsCount(), firstStat.GetTelemetrySamplesCount(), firstStat.GetUpdatedAt())
	submitResp, err := statsClient.SubmitUserStats(authCtx, firstStat)
	if err != nil {
		log.Fatalf("submit first stat: %v", err)
	}
	fmt.Println("SubmitUserStats response #1:")
	fmt.Printf("{success:%t, message:%q}\n\n", submitResp.GetSuccess(), submitResp.GetMessage())

	fmt.Println("SubmitUserStats request #2:")
	fmt.Printf("{vehicle_id:%q, brand:%q, model:%q, vehicle_type:%q, manual_avg_consumption_l_per_100km:%v, obd_avg_consumption_l_per_100km:%v, avg_refuel_liters:%v, avg_distance_between_refuels_km:%v, manual_records_count:%d, telemetry_samples_count:%d, updated_at:%q}\n",
		secondStat.GetVehicleId(), secondStat.GetBrand(), secondStat.GetModel(), secondStat.GetVehicleType(),
		secondStat.GetManualAvgConsumptionLPer_100Km(), secondStat.GetObdAvgConsumptionLPer_100Km(),
		secondStat.GetAvgRefuelLiters(), secondStat.GetAvgDistanceBetweenRefuelsKm(),
		secondStat.GetManualRecordsCount(), secondStat.GetTelemetrySamplesCount(), secondStat.GetUpdatedAt())
	submitResp, err = statsClient.SubmitUserStats(authCtx, secondStat)
	if err != nil {
		log.Fatalf("submit second stat: %v", err)
	}
	fmt.Println("SubmitUserStats response #2:")
	fmt.Printf("{success:%t, message:%q}\n\n", submitResp.GetSuccess(), submitResp.GetMessage())

	fmt.Println("GetGlobalStats request:")
	fmt.Printf("{brand:%q, model:%q}\n", brand, model)

	statsResp, err := statsClient.GetGlobalStats(authCtx, &pb.GetGlobalStatsRequest{
		Brand: brand,
		Model: model,
	})
	if err != nil {
		log.Fatalf("get global stats: %v", err)
	}

	fmt.Println("GetGlobalStats response:")
	fmt.Printf("{group_name:%q, manual_avg_consumption_l_per_100km:%v, obd_avg_consumption_l_per_100km:%v, avg_refuel_liters:%v, avg_distance_between_refuels_km:%v, vehicles_count:%d}\n",
		statsResp.GetGroupName(),
		statsResp.GetManualAvgConsumptionLPer_100Km(),
		statsResp.GetObdAvgConsumptionLPer_100Km(),
		statsResp.GetAvgRefuelLiters(),
		statsResp.GetAvgDistanceBetweenRefuelsKm(),
		statsResp.GetVehiclesCount(),
	)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
