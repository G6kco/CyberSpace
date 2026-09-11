CREATE TABLE assessments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    level_id BIGINT UNSIGNED NOT NULL,

    title VARCHAR(200) NOT NULL,
    slug VARCHAR(160) NOT NULL,
    description TEXT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'draft',

    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_assessments_level_slug
        UNIQUE (level_id, slug),

    CONSTRAINT chk_assessments_status
        CHECK (status IN ('draft', 'published', 'archived')),

    CONSTRAINT fk_assessments_level
        FOREIGN KEY (level_id)
        REFERENCES levels (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_assessments_created_by
        FOREIGN KEY (created_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_assessments_level_status (
        level_id,
        status
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;