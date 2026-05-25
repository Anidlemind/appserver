package main

import (
	"context"
	"log"
	"os"

	"carstats/internal/aggregation"
	"carstats/internal/db"

	"github.com/robfig/cron/v3"
)

func main() {

	pool, err := db.NewPool()
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	aggregator := aggregation.NewAggregator(pool)

	if err := aggregator.RefreshAll(context.Background()); err != nil {
		log.Fatal(err)
	}

	schedule := os.Getenv("AGGREGATION_SCHEDULE")
	if schedule == "" {
		schedule = "@every 5m"
	}

	c := cron.New()
	_, err = c.AddFunc(schedule, aggregator.Run)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("stats-aggregator started with schedule %s", schedule)
	c.Start()

	select {}
}
