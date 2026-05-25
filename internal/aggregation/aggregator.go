package aggregation

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Aggregator struct {
	db *pgxpool.Pool
}

func NewAggregator(pool *pgxpool.Pool) *Aggregator {
	return &Aggregator{db: pool}
}

func (a *Aggregator) Run() {
	if err := a.RefreshAll(context.Background()); err != nil {
		log.Printf("batch aggregation failed: %v", err)
		return
	}

	log.Println("batch aggregation completed")
}

func (a *Aggregator) RefreshAll(ctx context.Context) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `TRUNCATE aggregated_stats_by_model, aggregated_stats_by_vehicle_type`); err != nil {
		return err
	}

	if err := refreshModelAggregates(ctx, tx); err != nil {
		return err
	}
	if err := refreshVehicleTypeAggregates(ctx, tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (a *Aggregator) RefreshGroup(ctx context.Context, brand, model, vehicleType string) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	brand = strings.TrimSpace(brand)
	model = strings.TrimSpace(model)
	vehicleType = strings.TrimSpace(vehicleType)

	if brand != "" && model != "" {
		if err := refreshSingleModelAggregate(ctx, tx, brand, model); err != nil {
			return err
		}
	}
	if vehicleType != "" {
		if err := refreshSingleVehicleTypeAggregate(ctx, tx, vehicleType); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func refreshModelAggregates(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}) error {
	_, err := db.Exec(ctx, `
		INSERT INTO aggregated_stats_by_model (
			brand,
			model,
			group_name,
			manual_avg_consumption_l_per_100km,
			obd_avg_consumption_l_per_100km,
			avg_refuel_liters,
			avg_distance_between_refuels_km,
			vehicles_count,
			updated_at
		)
		SELECT
			brand,
			model,
			brand || ' ' || model,
			COALESCE(SUM(manual_avg_consumption_l_per_100km * manual_records_count) / NULLIF(SUM(manual_records_count), 0), 0),
			COALESCE(SUM(obd_avg_consumption_l_per_100km * telemetry_samples_count) / NULLIF(SUM(telemetry_samples_count), 0), 0),
			COALESCE(AVG(avg_refuel_liters), 0),
			COALESCE(AVG(avg_distance_between_refuels_km), 0),
			COUNT(*)::BIGINT,
			NOW()
		FROM user_vehicle_stats
		GROUP BY brand, model
	`)
	return err
}

func refreshVehicleTypeAggregates(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}) error {
	_, err := db.Exec(ctx, `
		INSERT INTO aggregated_stats_by_vehicle_type (
			vehicle_type,
			group_name,
			manual_avg_consumption_l_per_100km,
			obd_avg_consumption_l_per_100km,
			avg_refuel_liters,
			avg_distance_between_refuels_km,
			vehicles_count,
			updated_at
		)
		SELECT
			vehicle_type,
			vehicle_type,
			COALESCE(SUM(manual_avg_consumption_l_per_100km * manual_records_count) / NULLIF(SUM(manual_records_count), 0), 0),
			COALESCE(SUM(obd_avg_consumption_l_per_100km * telemetry_samples_count) / NULLIF(SUM(telemetry_samples_count), 0), 0),
			COALESCE(AVG(avg_refuel_liters), 0),
			COALESCE(AVG(avg_distance_between_refuels_km), 0),
			COUNT(*)::BIGINT,
			NOW()
		FROM user_vehicle_stats
		GROUP BY vehicle_type
	`)
	return err
}

func refreshSingleModelAggregate(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, brand, model string) error {
	if _, err := db.Exec(ctx, `DELETE FROM aggregated_stats_by_model WHERE brand = $1 AND model = $2`, brand, model); err != nil {
		return err
	}

	_, err := db.Exec(ctx, `
		INSERT INTO aggregated_stats_by_model (
			brand,
			model,
			group_name,
			manual_avg_consumption_l_per_100km,
			obd_avg_consumption_l_per_100km,
			avg_refuel_liters,
			avg_distance_between_refuels_km,
			vehicles_count,
			updated_at
		)
		SELECT
			brand,
			model,
			brand || ' ' || model,
			COALESCE(SUM(manual_avg_consumption_l_per_100km * manual_records_count) / NULLIF(SUM(manual_records_count), 0), 0),
			COALESCE(SUM(obd_avg_consumption_l_per_100km * telemetry_samples_count) / NULLIF(SUM(telemetry_samples_count), 0), 0),
			COALESCE(AVG(avg_refuel_liters), 0),
			COALESCE(AVG(avg_distance_between_refuels_km), 0),
			COUNT(*)::BIGINT,
			NOW()
		FROM user_vehicle_stats
		WHERE brand = $1 AND model = $2
		GROUP BY brand, model
	`, brand, model)

	return err
}

func refreshSingleVehicleTypeAggregate(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, vehicleType string) error {
	if _, err := db.Exec(ctx, `DELETE FROM aggregated_stats_by_vehicle_type WHERE vehicle_type = $1`, vehicleType); err != nil {
		return err
	}

	_, err := db.Exec(ctx, `
		INSERT INTO aggregated_stats_by_vehicle_type (
			vehicle_type,
			group_name,
			manual_avg_consumption_l_per_100km,
			obd_avg_consumption_l_per_100km,
			avg_refuel_liters,
			avg_distance_between_refuels_km,
			vehicles_count,
			updated_at
		)
		SELECT
			vehicle_type,
			vehicle_type,
			COALESCE(SUM(manual_avg_consumption_l_per_100km * manual_records_count) / NULLIF(SUM(manual_records_count), 0), 0),
			COALESCE(SUM(obd_avg_consumption_l_per_100km * telemetry_samples_count) / NULLIF(SUM(telemetry_samples_count), 0), 0),
			COALESCE(AVG(avg_refuel_liters), 0),
			COALESCE(AVG(avg_distance_between_refuels_km), 0),
			COUNT(*)::BIGINT,
			NOW()
		FROM user_vehicle_stats
		WHERE vehicle_type = $1
		GROUP BY vehicle_type
	`, vehicleType)

	return err
}

func QueryByModel(ctx context.Context, db *pgxpool.Pool, brand, model string) (GlobalStats, error) {
	return queryAggregate(
		ctx,
		db,
		`SELECT group_name, manual_avg_consumption_l_per_100km, obd_avg_consumption_l_per_100km, avg_refuel_liters, avg_distance_between_refuels_km, vehicles_count
		 FROM aggregated_stats_by_model
		 WHERE brand = $1 AND model = $2`,
		brand,
		model,
	)
}

func QueryByVehicleType(ctx context.Context, db *pgxpool.Pool, vehicleType string) (GlobalStats, error) {
	return queryAggregate(
		ctx,
		db,
		`SELECT group_name, manual_avg_consumption_l_per_100km, obd_avg_consumption_l_per_100km, avg_refuel_liters, avg_distance_between_refuels_km, vehicles_count
		 FROM aggregated_stats_by_vehicle_type
		 WHERE vehicle_type = $1`,
		vehicleType,
	)
}

type GlobalStats struct {
	GroupName                   string
	ManualAvg                   float64
	ObdAvg                      float64
	AvgRefuelLiters             float64
	AvgDistanceBetweenRefuelsKm float64
	VehiclesCount               int64
}

func queryAggregate(ctx context.Context, db *pgxpool.Pool, query string, args ...any) (GlobalStats, error) {
	var stats GlobalStats

	err := db.QueryRow(ctx, query, args...).Scan(
		&stats.GroupName,
		&stats.ManualAvg,
		&stats.ObdAvg,
		&stats.AvgRefuelLiters,
		&stats.AvgDistanceBetweenRefuelsKm,
		&stats.VehiclesCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return GlobalStats{}, nil
		}
		return GlobalStats{}, fmt.Errorf("query aggregate: %w", err)
	}

	return stats, nil
}
