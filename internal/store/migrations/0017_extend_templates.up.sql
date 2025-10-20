ALTER TABLE templates RENAME COLUMN spec TO base;

ALTER TABLE templates
    ADD COLUMN description TEXT NOT NULL DEFAULT '',
    ADD COLUMN schema JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN defaults JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN example JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN delivery_template TEXT NOT NULL DEFAULT '';

ALTER TABLE templates
    ALTER COLUMN description DROP DEFAULT,
    ALTER COLUMN schema DROP DEFAULT,
    ALTER COLUMN defaults DROP DEFAULT,
    ALTER COLUMN example DROP DEFAULT,
    ALTER COLUMN delivery_template DROP DEFAULT;
