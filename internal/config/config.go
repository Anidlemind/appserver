package config

import "time"

type Config struct {
	Env                 string
	GRPCAddr            string
	DatabaseURL         string
	AggregationSchedule string
	AuthTokenTTL        time.Duration
}
