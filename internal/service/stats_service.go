package service

import (
	"context"
	"time"

	pb "carstats/proto"
	"carstats/internal/sso"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StatsServer struct {
	pb.UnimplementedStatsServiceServer
	db *pgxpool.Pool
}

func NewStatsServer(db *pgxpool.Pool) *StatsServer {
	return &StatsServer{db: db}
}

func (s *StatsServer) SubmitUserStats(ctx context.Context, req *pb.SubmitUserStatsRequest) (*pb.SubmitUserStatsResponse, error) {
	if req.VehicleType == "" {
		return nil, status.Error(codes.InvalidArgument, "vehicle_type is required")
	}
	if req.FuelLiters < 0 {
		return nil, status.Error(codes.InvalidArgument, "fuel_liters must be non-negative")
	}
	if req.DistanceKm < 0 {
		return nil, status.Error(codes.InvalidArgument, "distance_km must be non-negative")
	}

	userID, ok := sso.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authenticated user is required")
	}

	recordedAt := time.Now().UTC()
	if req.RecordedAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.RecordedAt)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "recorded_at must be RFC3339")
		}
		recordedAt = parsed.UTC()
	}

	query := `
	INSERT INTO raw_vehicle_stats
	(user_id, vehicle_type, fuel_liters, distance_km, recorded_at)
	VALUES ($1, $2, $3, $4, $5)
	`

	_, err := s.db.Exec(ctx, query,
		userID,
		req.VehicleType,
		req.FuelLiters,
		req.DistanceKm,
		recordedAt,
	)
	if err != nil {
		return nil, err
	}

	return &pb.SubmitUserStatsResponse{
		Success: true,
		Message: "stat accepted into raw storage",
	}, nil
}

func (s *StatsServer) GetGlobalStats(ctx context.Context, req *pb.GetGlobalStatsRequest) (*pb.GetGlobalStatsResponse, error) {
	if req.VehicleType == "" {
		return nil, status.Error(codes.InvalidArgument, "vehicle_type is required")
	}

	query := `
	SELECT
		COALESCE(SUM(total_fuel_liters), 0),
		COALESCE(SUM(total_distance_km), 0),
		COALESCE(SUM(total_records), 0)
	FROM (
		SELECT total_fuel_liters, total_distance_km, total_records
		FROM vehicle_type_aggregates
		WHERE vehicle_type = $1

		UNION ALL

		SELECT
			COALESCE(SUM(fuel_liters), 0) AS total_fuel_liters,
			COALESCE(SUM(distance_km), 0) AS total_distance_km,
			COUNT(*)::BIGINT AS total_records
		FROM raw_vehicle_stats
		WHERE vehicle_type = $1
	) AS combined
	`

	var (
		totalFuelLiters  float64
		totalDistanceKm  float64
		totalRecords     int64
	)

	err := s.db.QueryRow(ctx, query,
		req.VehicleType,
	).Scan(
		&totalFuelLiters,
		&totalDistanceKm,
		&totalRecords,
	)

	if err != nil {
		return nil, err
	}

	avgFuelPerKm := 0.0
	if totalDistanceKm > 0 {
		avgFuelPerKm = totalFuelLiters / totalDistanceKm
	}

	return &pb.GetGlobalStatsResponse{
		VehicleType:     req.VehicleType,
		TotalFuelLiters: totalFuelLiters,
		TotalDistanceKm: totalDistanceKm,
		TotalRecords:    totalRecords,
		AvgFuelPerKm:    avgFuelPerKm,
	}, nil
}
