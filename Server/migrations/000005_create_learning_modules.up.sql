CREATE TABLE learning_modules (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    level_id BIGINT UNSIGNED NOT NULL,

    title VARCHAR(200) NOT NULL,
    slug VARCHAR(160) NOT NULL,
    description TEXT NULL,

    sequence_no SMALLINT UNSIGNED NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',

    published_at DATETIME(6) NULL,

    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_learning_modules_level_slug
        UNIQUE (level_id, slug),

    CONSTRAINT uq_learning_modules_level_sequence
        UNIQUE (level_id, sequence_no),

    CONSTRAINT chk_learning_modules_status
        CHECK (status IN ('draft', 'published', 'archived')),

    CONSTRAINT chk_learning_modules_publication
        CHECK (
            (status = 'draft' AND published_at IS NULL)
            OR
            (status IN ('published', 'archived') AND published_at IS NOT NULL)
        ),

    CONSTRAINT fk_learning_modules_level
        FOREIGN KEY (level_id)
        REFERENCES levels (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_learning_modules_created_by
        FOREIGN KEY (created_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_learning_modules_level_status (
        level_id,
        status,
        sequence_no
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;