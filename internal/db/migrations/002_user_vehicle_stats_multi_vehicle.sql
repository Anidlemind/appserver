ALTER TABLE user_vehicle_stats DROP CONSTRAINT IF EXISTS user_vehicle_stats_pkey;

ALTER TABLE user_vehicle_stats
	ADD CONSTRAINT user_vehicle_stats_pkey PRIMARY KEY (user_id, vehicle_id);

CREATE INDEX IF NOT EXISTS idx_user_vehicle_stats_user_id
	ON user_vehicle_stats(user_id);
