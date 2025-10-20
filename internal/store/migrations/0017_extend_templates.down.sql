ALTER TABLE templates
    RENAME COLUMN base TO spec;

ALTER TABLE templates
    DROP COLUMN description,
    DROP COLUMN schema,
    DROP COLUMN defaults,
    DROP COLUMN example,
    DROP COLUMN delivery_template;
