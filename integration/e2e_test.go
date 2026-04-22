package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	pb "carstats/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func TestAuthAndStatsFlow(t *testing.T) {
	addr := envOrDefault("TEST_GRPC_ADDR", "127.0.0.1:50051")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial grpc server: %v", err)
	}
	defer conn.Close()

	authClient := pb.NewAuthClient(conn)
	statsClient := pb.NewStatsServiceClient(conn)

	suffix := time.Now().UTC().Format("20060102150405.000000000")
	suffix = strings.ReplaceAll(suffix, ".", "")
	email := "e2e-" + suffix + "@example.com"
	password := "secret123"
	vehicleType := "sedan-e2e-" + suffix

	registerResp, err := authClient.Register(ctx, &pb.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if registerResp.GetUserId() == 0 {
		t.Fatal("register returned empty user id")
	}
	if registerResp.GetToken() == "" {
		t.Fatal("register returned empty token")
	}

	loginResp, err := authClient.Login(ctx, &pb.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if loginResp.GetToken() == "" {
		t.Fatal("login returned empty token")
	}

	authCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"authorization", "Bearer "+loginResp.GetToken(),
	))

	_, err = statsClient.SubmitUserStats(authCtx, &pb.SubmitUserStatsRequest{
		VehicleType: vehicleType,
		FuelLiters:  20.5,
		DistanceKm:  250,
		RecordedAt:  "2026-04-21T10:30:00Z",
	})
	if err != nil {
		t.Fatalf("submit first stat: %v", err)
	}

	_, err = statsClient.SubmitUserStats(authCtx, &pb.SubmitUserStatsRequest{
		VehicleType: vehicleType,
		FuelLiters:  10,
		DistanceKm:  100,
		RecordedAt:  "2026-04-21T10:35:00Z",
	})
	if err != nil {
		t.Fatalf("submit second stat: %v", err)
	}

	statsResp, err := statsClient.GetGlobalStats(authCtx, &pb.GetGlobalStatsRequest{
		VehicleType: vehicleType,
	})
	if err != nil {
		t.Fatalf("get global stats: %v", err)
	}

	if statsResp.GetVehicleType() != vehicleType {
		t.Fatalf("unexpected vehicle type: %q", statsResp.GetVehicleType())
	}
	if statsResp.GetTotalFuelLiters() != 30.5 {
		t.Fatalf("unexpected total fuel: got %v want 30.5", statsResp.GetTotalFuelLiters())
	}
	if statsResp.GetTotalDistanceKm() != 350 {
		t.Fatalf("unexpected total distance: got %v want 350", statsResp.GetTotalDistanceKm())
	}
	if statsResp.GetTotalRecords() != 2 {
		t.Fatalf("unexpected total records: got %v want 2", statsResp.GetTotalRecords())
	}

	const expectedAvg = 30.5 / 350.0
	if diff := statsResp.GetAvgFuelPerKm() - expectedAvg; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("unexpected avg fuel per km: got %.12f want %.12f", statsResp.GetAvgFuelPerKm(), expectedAvg)
	}
}

func TestStatsRequireAuth(t *testing.T) {
	addr := envOrDefault("TEST_GRPC_ADDR", "127.0.0.1:50051")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial grpc server: %v", err)
	}
	defer conn.Close()

	statsClient := pb.NewStatsServiceClient(conn)

	_, err = statsClient.GetGlobalStats(ctx, &pb.GetGlobalStatsRequest{
		VehicleType: "sedan-no-auth",
	})
	if err == nil {
		t.Fatal("expected unauthenticated error without token")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "unauthenticated") {
		t.Fatalf("unexpected error without token: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
