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

const floatTolerance = 1e-9

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
	uniqueID := "e2e-" + suffix
	vehicleType := "sedan"
	brand := "Kia"
	model := "Rio-" + suffix
	secondUniqueID := "e2e-second-" + suffix

	registerResp, err := authClient.Register(ctx, &pb.RegisterRequest{
		UniqueId: uniqueID,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if registerResp.GetToken() == "" {
		t.Fatal("register returned empty token")
	}

	loginResp, err := authClient.Login(ctx, &pb.LoginRequest{
		UniqueId: uniqueID,
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
		VehicleId:                        "vehicle-a-" + suffix,
		Brand:                            brand,
		Model:                            model,
		VehicleType:                      vehicleType,
		ManualAvgConsumptionLPer_100Km:   8.0,
		ObdAvgConsumptionLPer_100Km:      8.4,
		AvgRefuelLiters:                  34.0,
		AvgDistanceBetweenRefuelsKm:      400,
		ManualRecordsCount:               4,
		TelemetrySamplesCount:            100,
		UpdatedAt:                        "2026-04-21T10:30:00Z",
	})
	if err != nil {
		t.Fatalf("submit initial stat: %v", err)
	}

	_, err = statsClient.SubmitUserStats(authCtx, &pb.SubmitUserStatsRequest{
		VehicleId:                        "vehicle-b-" + suffix,
		Brand:                            brand,
		Model:                            model,
		VehicleType:                      vehicleType,
		ManualAvgConsumptionLPer_100Km:   9.0,
		ObdAvgConsumptionLPer_100Km:      9.3,
		AvgRefuelLiters:                  40.0,
		AvgDistanceBetweenRefuelsKm:      460,
		ManualRecordsCount:               10,
		TelemetrySamplesCount:            300,
		UpdatedAt:                        "2026-04-21T10:35:00Z",
	})
	if err != nil {
		t.Fatalf("submit updated stat: %v", err)
	}

	secondRegisterResp, err := authClient.Register(ctx, &pb.RegisterRequest{
		UniqueId: secondUniqueID,
	})
	if err != nil {
		t.Fatalf("second register: %v", err)
	}
	if secondRegisterResp.GetToken() == "" {
		t.Fatal("second register returned empty token")
	}

	secondAuthCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"authorization", "Bearer "+secondRegisterResp.GetToken(),
	))

	_, err = statsClient.SubmitUserStats(secondAuthCtx, &pb.SubmitUserStatsRequest{
		VehicleId:                        "vehicle-c-" + suffix,
		Brand:                            brand,
		Model:                            model,
		VehicleType:                      vehicleType,
		ManualAvgConsumptionLPer_100Km:   7.0,
		ObdAvgConsumptionLPer_100Km:      8.1,
		AvgRefuelLiters:                  30.0,
		AvgDistanceBetweenRefuelsKm:      390,
		ManualRecordsCount:               2,
		TelemetrySamplesCount:            50,
		UpdatedAt:                        "2026-04-21T10:40:00Z",
	})
	if err != nil {
		t.Fatalf("submit second user stat: %v", err)
	}

	statsResp, err := statsClient.GetGlobalStats(authCtx, &pb.GetGlobalStatsRequest{
		Brand: brand,
		Model: model,
	})
	if err != nil {
		t.Fatalf("get global stats: %v", err)
	}

	if statsResp.GetGroupName() != brand+" "+model {
		t.Fatalf("unexpected group name: %q", statsResp.GetGroupName())
	}
	if diff := statsResp.GetManualAvgConsumptionLPer_100Km() - (104.0 / 12.0); diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("unexpected manual avg: got %.12f want %.12f", statsResp.GetManualAvgConsumptionLPer_100Km(), 104.0/12.0)
	}
	if diff := statsResp.GetObdAvgConsumptionLPer_100Km() - (3195.0 / 350.0); diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("unexpected obd avg: got %.12f want %.12f", statsResp.GetObdAvgConsumptionLPer_100Km(), 3195.0/350.0)
	}
	if statsResp.GetAvgRefuelLiters() != 35.0 {
		t.Fatalf("unexpected avg refuel liters: got %v want 35.0", statsResp.GetAvgRefuelLiters())
	}
	if statsResp.GetAvgDistanceBetweenRefuelsKm() != 425 {
		t.Fatalf("unexpected avg distance between refuels: got %v want 425", statsResp.GetAvgDistanceBetweenRefuelsKm())
	}
	if statsResp.GetVehiclesCount() != 2 {
		t.Fatalf("unexpected vehicles count: got %v want 2", statsResp.GetVehiclesCount())
	}

	typeStatsResp, err := statsClient.GetGlobalStats(authCtx, &pb.GetGlobalStatsRequest{
		VehicleType: vehicleType,
	})
	if err != nil {
		t.Fatalf("get type stats: %v", err)
	}
	if typeStatsResp.GetGroupName() != vehicleType {
		t.Fatalf("unexpected type group name: %q", typeStatsResp.GetGroupName())
	}
	if typeStatsResp.GetVehiclesCount() != 2 {
		t.Fatalf("unexpected type vehicles count: got %v want 2", typeStatsResp.GetVehiclesCount())
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
		VehicleType: "sedan",
	})
	if err == nil {
		t.Fatal("expected unauthenticated error without token")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "unauthenticated") {
		t.Fatalf("unexpected error without token: %v", err)
	}
}

func TestStatsAggregationUsesLatestUserStateAndWeightedAverages(t *testing.T) {
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

	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
	brand := "Toyota"
	model := "Corolla-" + suffix
	vehicleType := "sedan-agg-" + suffix

	authCtx1 := mustRegisterAndAuth(t, ctx, authClient, "agg1-"+suffix)
	authCtx2 := mustRegisterAndAuth(t, ctx, authClient, "agg2-"+suffix)
	authCtx3 := mustRegisterAndAuth(t, ctx, authClient, "agg3-"+suffix)

	mustSubmitStats(t, authCtx1, statsClient, &pb.SubmitUserStatsRequest{
		VehicleId:                      "agg-user-1-old",
		Brand:                          brand,
		Model:                          model,
		VehicleType:                    vehicleType,
		ManualAvgConsumptionLPer_100Km: 12.0,
		ObdAvgConsumptionLPer_100Km:    12.5,
		AvgRefuelLiters:                52.0,
		AvgDistanceBetweenRefuelsKm:    500.0,
		ManualRecordsCount:             100,
		TelemetrySamplesCount:          1000,
		UpdatedAt:                      "2026-04-24T18:00:00Z",
	})

	mustSubmitStats(t, authCtx1, statsClient, &pb.SubmitUserStatsRequest{
		VehicleId:                      "agg-user-1-new",
		Brand:                          brand,
		Model:                          model,
		VehicleType:                    vehicleType,
		ManualAvgConsumptionLPer_100Km: 8.0,
		ObdAvgConsumptionLPer_100Km:    8.5,
		AvgRefuelLiters:                40.0,
		AvgDistanceBetweenRefuelsKm:    420.0,
		ManualRecordsCount:             4,
		TelemetrySamplesCount:          100,
		UpdatedAt:                      "2026-04-24T19:00:00Z",
	})

	mustSubmitStats(t, authCtx2, statsClient, &pb.SubmitUserStatsRequest{
		VehicleId:                      "agg-user-2",
		Brand:                          brand,
		Model:                          model,
		VehicleType:                    vehicleType,
		ManualAvgConsumptionLPer_100Km: 10.0,
		ObdAvgConsumptionLPer_100Km:    9.0,
		AvgRefuelLiters:                50.0,
		AvgDistanceBetweenRefuelsKm:    600.0,
		ManualRecordsCount:             6,
		TelemetrySamplesCount:          300,
		UpdatedAt:                      "2026-04-24T19:10:00Z",
	})

	mustSubmitStats(t, authCtx3, statsClient, &pb.SubmitUserStatsRequest{
		VehicleId:                      "agg-user-3",
		Brand:                          "Honda",
		Model:                          "Civic-" + suffix,
		VehicleType:                    vehicleType,
		ManualAvgConsumptionLPer_100Km: 99.0,
		ObdAvgConsumptionLPer_100Km:    99.0,
		AvgRefuelLiters:                60.0,
		AvgDistanceBetweenRefuelsKm:    300.0,
		ManualRecordsCount:             0,
		TelemetrySamplesCount:          0,
		UpdatedAt:                      "2026-04-24T19:20:00Z",
	})

	modelStats, err := statsClient.GetGlobalStats(authCtx1, &pb.GetGlobalStatsRequest{
		Brand: brand,
		Model: model,
	})
	if err != nil {
		t.Fatalf("get model stats: %v", err)
	}

	if modelStats.GetGroupName() != brand+" "+model {
		t.Fatalf("unexpected model group name: %q", modelStats.GetGroupName())
	}
	assertFloatApproxEqual(t, modelStats.GetManualAvgConsumptionLPer_100Km(), 9.2, "model manual avg")
	assertFloatApproxEqual(t, modelStats.GetObdAvgConsumptionLPer_100Km(), 8.875, "model obd avg")
	assertFloatApproxEqual(t, modelStats.GetAvgRefuelLiters(), 45.0, "model avg refuel liters")
	assertFloatApproxEqual(t, modelStats.GetAvgDistanceBetweenRefuelsKm(), 510.0, "model avg distance between refuels")
	if modelStats.GetVehiclesCount() != 2 {
		t.Fatalf("unexpected model vehicles count: got %v want 2", modelStats.GetVehiclesCount())
	}

	typeStats, err := statsClient.GetGlobalStats(authCtx1, &pb.GetGlobalStatsRequest{
		VehicleType: vehicleType,
	})
	if err != nil {
		t.Fatalf("get type stats: %v", err)
	}

	if typeStats.GetGroupName() != vehicleType {
		t.Fatalf("unexpected type group name: %q", typeStats.GetGroupName())
	}
	assertFloatApproxEqual(t, typeStats.GetManualAvgConsumptionLPer_100Km(), 9.2, "type manual avg")
	assertFloatApproxEqual(t, typeStats.GetObdAvgConsumptionLPer_100Km(), 8.875, "type obd avg")
	assertFloatApproxEqual(t, typeStats.GetAvgRefuelLiters(), 50.0, "type avg refuel liters")
	assertFloatApproxEqual(t, typeStats.GetAvgDistanceBetweenRefuelsKm(), 440.0, "type avg distance between refuels")
	if typeStats.GetVehiclesCount() != 3 {
		t.Fatalf("unexpected type vehicles count: got %v want 3", typeStats.GetVehiclesCount())
	}
}

func mustRegisterAndAuth(t *testing.T, ctx context.Context, authClient pb.AuthClient, uniqueID string) context.Context {
	t.Helper()

	registerResp, err := authClient.Register(ctx, &pb.RegisterRequest{
		UniqueId: uniqueID,
	})
	if err != nil {
		t.Fatalf("register %s: %v", uniqueID, err)
	}
	if registerResp.GetToken() == "" {
		t.Fatalf("register %s returned empty token", uniqueID)
	}

	return metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"authorization", "Bearer "+registerResp.GetToken(),
	))
}

func mustSubmitStats(t *testing.T, ctx context.Context, statsClient pb.StatsServiceClient, req *pb.SubmitUserStatsRequest) {
	t.Helper()

	if _, err := statsClient.SubmitUserStats(ctx, req); err != nil {
		t.Fatalf("submit stats for vehicle %s: %v", req.GetVehicleId(), err)
	}
}

func assertFloatApproxEqual(t *testing.T, got, want float64, label string) {
	t.Helper()

	diff := got - want
	if diff < -floatTolerance || diff > floatTolerance {
		t.Fatalf("%s: got %.12f want %.12f", label, got, want)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
