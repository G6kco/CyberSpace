CREATE TABLE lab_template_versions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    lab_template_id BIGINT UNSIGNED NOT NULL,
    version_no INT UNSIGNED NOT NULL,

    yaml_spec MEDIUMTEXT NOT NULL,
    spec_sha256 BINARY(32) NOT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    published_at DATETIME(6) NULL,

    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    published_template_id BIGINT UNSIGNED
        GENERATED ALWAYS AS (
            CASE
                WHEN status = 'published' THEN lab_template_id
                ELSE NULL
            END
        ) STORED,

    PRIMARY KEY (id),

    CONSTRAINT uq_lab_template_versions_number
        UNIQUE (lab_template_id, version_no),

    CONSTRAINT uq_lab_template_versions_spec
        UNIQUE (lab_template_id, spec_sha256),

    CONSTRAINT uq_lab_template_versions_one_published
        UNIQUE (published_template_id),

    CONSTRAINT chk_lab_template_versions_yaml
        CHECK (CHAR_LENGTH(TRIM(yaml_spec)) > 0),

    CONSTRAINT chk_lab_template_versions_status
        CHECK (
            status IN ('draft', 'published', 'retired')
        ),

    CONSTRAINT chk_lab_template_versions_publication
        CHECK (
            (status = 'draft' AND published_at IS NULL)
            OR
            (
                status IN ('published', 'retired')
                AND published_at IS NOT NULL
            )
        ),

    CONSTRAINT fk_lab_template_versions_template
    FOREIGN KEY (lab_template_id)
    REFERENCES lab_templates (id)
    ON UPDATE RESTRICT
    ON DELETE RESTRICT,

    CONSTRAINT fk_lab_template_versions_created_by
        FOREIGN KEY (created_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_lab_template_versions_template_status (
        lab_template_id,
        status
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;