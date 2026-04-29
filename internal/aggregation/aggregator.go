package aggregation

import (
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Aggregator struct{}

func NewAggregator(_ *pgxpool.Pool) *Aggregator {
	return &Aggregator{}
}

func (a *Aggregator) Run() {
	log.Println("Batch aggregation skipped: stats are served directly from user_vehicle_stats")
}
