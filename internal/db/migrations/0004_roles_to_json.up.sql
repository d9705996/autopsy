-- 0004_roles_to_json.up.sql
-- Convert roles column from TEXT[] to TEXT (JSON array) for GORM cross-driver compatibility.
ALTER TABLE users
    ALTER COLUMN roles TYPE TEXT
        USING array_to_json(roles)::TEXT;

ALTER TABLE users
    ALTER COLUMN roles SET DEFAULT '[]';
