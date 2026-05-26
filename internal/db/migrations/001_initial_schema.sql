CREATE TABLE IF NOT EXISTS users (
	id BIGSERIAL PRIMARY KEY,
	external_id TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS auth_tokens (
	token TEXT PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_id
	ON auth_tokens(user_id);

CREATE TABLE IF NOT EXISTS user_vehicle_stats (
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	vehicle_id TEXT NOT NULL,
	brand TEXT NOT NULL,
	model TEXT NOT NULL,
	vehicle_type TEXT NOT NULL,
	manual_avg_consumption_l_per_100km DOUBLE PRECISION NOT NULL CHECK (manual_avg_consumption_l_per_100km >= 0),
	obd_avg_consumption_l_per_100km DOUBLE PRECISION NOT NULL CHECK (obd_avg_consumption_l_per_100km >= 0),
	avg_refuel_liters DOUBLE PRECISION NOT NULL CHECK (avg_refuel_liters >= 0),
	avg_distance_between_refuels_km DOUBLE PRECISION NOT NULL CHECK (avg_distance_between_refuels_km >= 0),
	manual_records_count BIGINT NOT NULL CHECK (manual_records_count >= 0),
	telemetry_samples_count BIGINT NOT NULL CHECK (telemetry_samples_count >= 0),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (user_id, vehicle_id)
);

CREATE INDEX IF NOT EXISTS idx_user_vehicle_stats_user_id
	ON user_vehicle_stats(user_id);

CREATE INDEX IF NOT EXISTS idx_user_vehicle_stats_brand_model
	ON user_vehicle_stats(brand, model);

CREATE INDEX IF NOT EXISTS idx_user_vehicle_stats_vehicle_type
	ON user_vehicle_stats(vehicle_type);

CREATE TABLE IF NOT EXISTS aggregated_stats_by_model (
	brand TEXT NOT NULL,
	model TEXT NOT NULL,
	group_name TEXT NOT NULL,
	manual_avg_consumption_l_per_100km DOUBLE PRECISION NOT NULL,
	obd_avg_consumption_l_per_100km DOUBLE PRECISION NOT NULL,
	avg_refuel_liters DOUBLE PRECISION NOT NULL,
	avg_distance_between_refuels_km DOUBLE PRECISION NOT NULL,
	vehicles_count BIGINT NOT NULL CHECK (vehicles_count >= 0),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (brand, model)
);

CREATE TABLE IF NOT EXISTS aggregated_stats_by_vehicle_type (
	vehicle_type TEXT PRIMARY KEY,
	group_name TEXT NOT NULL,
	manual_avg_consumption_l_per_100km DOUBLE PRECISION NOT NULL,
	obd_avg_consumption_l_per_100km DOUBLE PRECISION NOT NULL,
	avg_refuel_liters DOUBLE PRECISION NOT NULL,
	avg_distance_between_refuels_km DOUBLE PRECISION NOT NULL,
	vehicles_count BIGINT NOT NULL CHECK (vehicles_count >= 0),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
