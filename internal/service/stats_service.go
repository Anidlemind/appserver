package service

import (
	"context"
	"strings"
	"time"

	"carstats/internal/aggregation"
	"carstats/internal/sso"
	pb "carstats/proto"

	"github.com/jackc/pgx/v5"
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

	newBrand := strings.TrimSpace(req.Brand)
	newModel := strings.TrimSpace(req.Model)
	newVehicleID := strings.TrimSpace(req.VehicleId)
	newVehicleType := strings.TrimSpace(req.VehicleType)
	if newVehicleType == "" {
		return nil, status.Error(codes.InvalidArgument, "vehicle_type is required")
	}

	var previous struct {
		brand       string
		model       string
		vehicleType string
	}

	err := s.db.QueryRow(
		ctx,
		`SELECT brand, model, vehicle_type FROM user_vehicle_stats WHERE user_id = $1 AND vehicle_id = $2`,
		userID,
		newVehicleID,
	).Scan(&previous.brand, &previous.model, &previous.vehicleType)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
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
	ON CONFLICT (user_id, vehicle_id) DO UPDATE SET
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
		newVehicleID,
		newBrand,
		newModel,
		newVehicleType,
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

	aggregator := aggregation.NewAggregator(s.db)

	if err := aggregator.RefreshGroup(ctx, newBrand, newModel, newVehicleType); err != nil {
		return nil, err
	}
	if (previous.brand != "" || previous.model != "" || previous.vehicleType != "") &&
		(previous.brand != newBrand || previous.model != newModel || previous.vehicleType != newVehicleType) {
		if err := aggregator.RefreshGroup(ctx, previous.brand, previous.model, previous.vehicleType); err != nil {
			return nil, err
		}
	}

	return &pb.SubmitUserStatsResponse{
		Success: true,
		Message: "user stats upserted",
	}, nil
}

func (s *StatsServer) GetGlobalStats(ctx context.Context, req *pb.GetGlobalStatsRequest) (*pb.GetGlobalStatsResponse, error) {
	var stats aggregation.GlobalStats

	brand := strings.TrimSpace(req.Brand)
	model := strings.TrimSpace(req.Model)
	vehicleType := strings.TrimSpace(req.VehicleType)

	switch {
	case brand != "" || model != "":
		if brand == "" || model == "" {
			return nil, status.Error(codes.InvalidArgument, "brand and model are required together")
		}
		var err error
		stats, err = aggregation.QueryByModel(ctx, s.db, brand, model)
		if err != nil {
			return nil, err
		}
		if stats.GroupName == "" {
			stats.GroupName = brand + " " + model
		}
	case vehicleType != "":
		var err error
		stats, err = aggregation.QueryByVehicleType(ctx, s.db, vehicleType)
		if err != nil {
			return nil, err
		}
		if stats.GroupName == "" {
			stats.GroupName = vehicleType
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "either brand/model or vehicle_type is required")
	}

	return &pb.GetGlobalStatsResponse{
		GroupName:                      stats.GroupName,
		ManualAvgConsumptionLPer_100Km: stats.ManualAvg,
		ObdAvgConsumptionLPer_100Km:    stats.ObdAvg,
		AvgRefuelLiters:                stats.AvgRefuelLiters,
		AvgDistanceBetweenRefuelsKm:    stats.AvgDistanceBetweenRefuelsKm,
		VehiclesCount:                  stats.VehiclesCount,
	}, nil
}
