CREATE TABLE IF NOT EXISTS entities
(
    id          BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    external_id UUID NOT NULL
);

DO $$
BEGIN
    FOR _ IN 1..15000 LOOP
        INSERT INTO entities (external_id)
        VALUES (gen_random_uuid());
    END LOOP;
END;
$$;

