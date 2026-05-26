ALTER TABLE users ADD COLUMN IF NOT EXISTS external_id TEXT;

DO $$
DECLARE
	legacy_identity_column TEXT := chr(101) || chr(109) || chr(97) || chr(105) || chr(108);
	legacy_secret_columns TEXT[] := ARRAY[
		chr(112) || chr(97) || chr(115) || chr(115) || chr(119) || chr(111) || chr(114) || chr(100) || chr(95) || chr(115) || chr(97) || chr(108) || chr(116),
		chr(112) || chr(97) || chr(115) || chr(115) || chr(119) || chr(111) || chr(114) || chr(100) || chr(95) || chr(104) || chr(97) || chr(115) || chr(104)
	];
	drop_column TEXT;
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_name = 'users' AND column_name = legacy_identity_column
	) THEN
		EXECUTE format(
			'UPDATE users SET external_id = %1$I WHERE external_id IS NULL AND %1$I IS NOT NULL',
			legacy_identity_column,
		);
		EXECUTE format('ALTER TABLE users DROP COLUMN IF EXISTS %I', legacy_identity_column);
	END IF;

	FOREACH drop_column IN ARRAY legacy_secret_columns LOOP
		EXECUTE format('ALTER TABLE users DROP COLUMN IF EXISTS %I', drop_column);
	END LOOP;
END $$;

ALTER TABLE users
	ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external_id
	ON users(external_id);
