package service

import (
	"context"
	"fmt"
	"strings"
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
	if strings.TrimSpace(req.VehicleId) == "" {
		return nil, status.Error(codes.InvalidArgument, "vehicle_id is required")
	}
	if strings.TrimSpace(req.Brand) == "" {
		return nil, status.Error(codes.InvalidArgument, "brand is required")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, status.Error(codes.InvalidArgument, "model is required")
	}
	if req.VehicleType == "" {
		return nil, status.Error(codes.InvalidArgument, "vehicle_type is required")
	}
	if req.ManualAvgConsumptionLPer_100Km < 0 {
		return nil, status.Error(codes.InvalidArgument, "manual_avg_consumption_l_per_100km must be non-negative")
	}
	if req.ObdAvgConsumptionLPer_100Km < 0 {
		return nil, status.Error(codes.InvalidArgument, "obd_avg_consumption_l_per_100km must be non-negative")
	}
	if req.AvgRefuelLiters < 0 {
		return nil, status.Error(codes.InvalidArgument, "avg_refuel_liters must be non-negative")
	}
	if req.AvgDistanceBetweenRefuelsKm < 0 {
		return nil, status.Error(codes.InvalidArgument, "avg_distance_between_refuels_km must be non-negative")
	}
	if req.ManualRecordsCount < 0 {
		return nil, status.Error(codes.InvalidArgument, "manual_records_count must be non-negative")
	}
	if req.TelemetrySamplesCount < 0 {
		return nil, status.Error(codes.InvalidArgument, "telemetry_samples_count must be non-negative")
	}

	userID, ok := sso.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authenticated user is required")
	}

	updatedAt := time.Now().UTC()
	if req.UpdatedAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.UpdatedAt)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "updated_at must be RFC3339")
		}
		updatedAt = parsed.UTC()
	}

	query := `
	INSERT INTO user_vehicle_stats (
		user_id,
		vehicle_id,
		brand,
		model,
		vehicle_type,
		manual_avg_consumption_l_per_100km,
		obd_avg_consumption_l_per_100km,
		avg_refuel_liters,
		avg_distance_between_refuels_km,
		manual_records_count,
		telemetry_samples_count,
		updated_at
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	ON CONFLICT (user_id) DO UPDATE SET
		vehicle_id = EXCLUDED.vehicle_id,
		brand = EXCLUDED.brand,
		model = EXCLUDED.model,
		vehicle_type = EXCLUDED.vehicle_type,
		manual_avg_consumption_l_per_100km = EXCLUDED.manual_avg_consumption_l_per_100km,
		obd_avg_consumption_l_per_100km = EXCLUDED.obd_avg_consumption_l_per_100km,
		avg_refuel_liters = EXCLUDED.avg_refuel_liters,
		avg_distance_between_refuels_km = EXCLUDED.avg_distance_between_refuels_km,
		manual_records_count = EXCLUDED.manual_records_count,
		telemetry_samples_count = EXCLUDED.telemetry_samples_count,
		updated_at = EXCLUDED.updated_at
	`

	_, err := s.db.Exec(ctx, query,
		userID,
		strings.TrimSpace(req.VehicleId),
		strings.TrimSpace(req.Brand),
		strings.TrimSpace(req.Model),
		strings.TrimSpace(req.VehicleType),
		req.ManualAvgConsumptionLPer_100Km,
		req.ObdAvgConsumptionLPer_100Km,
		req.AvgRefuelLiters,
		req.AvgDistanceBetweenRefuelsKm,
		req.ManualRecordsCount,
		req.TelemetrySamplesCount,
		updatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &pb.SubmitUserStatsResponse{
		Success: true,
		Message: "user stats upserted",
	}, nil
}

func (s *StatsServer) GetGlobalStats(ctx context.Context, req *pb.GetGlobalStatsRequest) (*pb.GetGlobalStatsResponse, error) {
	var (
		groupName string
		filterSQL string
		args      []any
	)

	brand := strings.TrimSpace(req.Brand)
	model := strings.TrimSpace(req.Model)
	vehicleType := strings.TrimSpace(req.VehicleType)

	switch {
	case brand != "" || model != "":
		if brand == "" || model == "" {
			return nil, status.Error(codes.InvalidArgument, "brand and model are required together")
		}
		groupName = fmt.Sprintf("%s %s", brand, model)
		filterSQL = "brand = $1 AND model = $2"
		args = []any{brand, model}
	case vehicleType != "":
		groupName = vehicleType
		filterSQL = "vehicle_type = $1"
		args = []any{vehicleType}
	default:
		return nil, status.Error(codes.InvalidArgument, "either brand/model or vehicle_type is required")
	}

	query := fmt.Sprintf(`
	SELECT
		COALESCE(SUM(manual_avg_consumption_l_per_100km * manual_records_count) / NULLIF(SUM(manual_records_count), 0), 0),
		COALESCE(SUM(obd_avg_consumption_l_per_100km * telemetry_samples_count) / NULLIF(SUM(telemetry_samples_count), 0), 0),
		COALESCE(AVG(avg_refuel_liters), 0),
		COALESCE(AVG(avg_distance_between_refuels_km), 0),
		COUNT(*)::BIGINT
	FROM user_vehicle_stats
	WHERE %s
	`, filterSQL)

	var (
		manualAvg                    float64
		obdAvg                       float64
		avgRefuelLiters              float64
		avgDistanceBetweenRefuelsKm  float64
		vehiclesCount                int64
	)

	err := s.db.QueryRow(ctx, query, args...).Scan(
		&manualAvg,
		&obdAvg,
		&avgRefuelLiters,
		&avgDistanceBetweenRefuelsKm,
		&vehiclesCount,
	)

	if err != nil {
		return nil, err
	}

	return &pb.GetGlobalStatsResponse{
		GroupName:                         groupName,
		ManualAvgConsumptionLPer_100Km:   manualAvg,
		ObdAvgConsumptionLPer_100Km:      obdAvg,
		AvgRefuelLiters:                  avgRefuelLiters,
		AvgDistanceBetweenRefuelsKm:      avgDistanceBetweenRefuelsKm,
		VehiclesCount:                    vehiclesCount,
	}, nil
}
