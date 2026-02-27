-- 0004_roles_to_json.down.sql
-- Revert roles column from TEXT (JSON) back to TEXT[] (Postgres array).
ALTER TABLE users
    ALTER COLUMN roles TYPE TEXT[]
        USING ARRAY(SELECT json_array_elements_text(roles::json));

ALTER TABLE users
    ALTER COLUMN roles SET DEFAULT '{}';
