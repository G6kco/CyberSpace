CREATE TABLE levels (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    name VARCHAR(120) NOT NULL,
    slug VARCHAR(120) NOT NULL,
    description TEXT NULL,

    sequence_no SMALLINT UNSIGNED NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_levels_slug
        UNIQUE (slug),

    CONSTRAINT uq_levels_sequence_no
        UNIQUE (sequence_no),

    CONSTRAINT chk_levels_status
        CHECK (status IN ('draft', 'published', 'archived'))
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;