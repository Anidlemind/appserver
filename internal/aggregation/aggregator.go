package aggregation

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Aggregator struct {
	db *pgxpool.Pool
}

func NewAggregator(db *pgxpool.Pool) *Aggregator {
	return &Aggregator{db: db}
}

func (a *Aggregator) Run() {

	log.Println("Running batch aggregation...")

	tx, err := a.db.Begin(context.Background())
	if err != nil {
		log.Println(err)
		return
	}
	defer tx.Rollback(context.Background())

	_, err = tx.Exec(context.Background(), `
		WITH moved AS (
			DELETE FROM raw_vehicle_stats
			WHERE id IN (
				SELECT id
				FROM raw_vehicle_stats
				FOR UPDATE SKIP LOCKED
			)
			RETURNING vehicle_type, fuel_liters, distance_km
		)
		INSERT INTO vehicle_type_aggregates (
			vehicle_type,
			total_fuel_liters,
			total_distance_km,
			total_records,
			updated_at
		)
		SELECT
			vehicle_type,
			SUM(fuel_liters),
			SUM(distance_km),
			COUNT(*),
			NOW()
		FROM moved
		GROUP BY vehicle_type
		ON CONFLICT (vehicle_type)
		DO UPDATE SET
			total_fuel_liters = vehicle_type_aggregates.total_fuel_liters + EXCLUDED.total_fuel_liters,
			total_distance_km = vehicle_type_aggregates.total_distance_km + EXCLUDED.total_distance_km,
			total_records = vehicle_type_aggregates.total_records + EXCLUDED.total_records,
			updated_at = NOW();
	`)

	if err != nil {
		log.Println(err)
		return
	}

	if err := tx.Commit(context.Background()); err != nil {
		log.Println(err)
		return
	}

	log.Println("Aggregation complete")
}
