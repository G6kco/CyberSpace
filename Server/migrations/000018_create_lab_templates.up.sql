CREATE TABLE lab_templates (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    name VARCHAR(200) NOT NULL,
    slug VARCHAR(160) NOT NULL,
    description TEXT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'draft',

    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_lab_templates_slug
        UNIQUE (slug),

    CONSTRAINT chk_lab_templates_status
        CHECK (
            status IN ('draft', 'active', 'archived')
        ),

    CONSTRAINT fk_lab_templates_created_by
        FOREIGN KEY (created_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_lab_templates_status (
        status
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;