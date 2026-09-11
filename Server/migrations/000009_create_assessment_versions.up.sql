CREATE TABLE assessment_versions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    assessment_id BIGINT UNSIGNED NOT NULL,
    version_no INT UNSIGNED NOT NULL,

    instructions MEDIUMTEXT NOT NULL,

    duration_seconds INT UNSIGNED NOT NULL,
    pass_score DECIMAL(10,2) UNSIGNED NOT NULL,
    total_points DECIMAL(10,2) UNSIGNED NOT NULL DEFAULT 0.00,
    max_attempts SMALLINT UNSIGNED NOT NULL DEFAULT 1,

    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    published_at DATETIME(6) NULL,

    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    published_assessment_id BIGINT UNSIGNED
        GENERATED ALWAYS AS (
            CASE
                WHEN status = 'published' THEN assessment_id
                ELSE NULL
            END
        ) STORED,

    PRIMARY KEY (id),

    CONSTRAINT uq_assessment_versions_number
        UNIQUE (assessment_id, version_no),

    CONSTRAINT uq_assessment_versions_one_published
        UNIQUE (published_assessment_id),

    CONSTRAINT chk_assessment_versions_duration
        CHECK (duration_seconds > 0),

    CONSTRAINT chk_assessment_versions_attempts
        CHECK (max_attempts > 0),

    CONSTRAINT chk_assessment_versions_scores
        CHECK (
            total_points >= 0.00
            AND pass_score >= 0.00
            AND pass_score <= total_points
        ),

    CONSTRAINT chk_assessment_versions_status
        CHECK (status IN ('draft', 'published', 'retired')),

    CONSTRAINT chk_assessment_versions_publication
        CHECK (
            (status = 'draft' AND published_at IS NULL)
            OR
            (
                status IN ('published', 'retired')
                AND published_at IS NOT NULL
                AND total_points > 0.00
            )
        ),

    CONSTRAINT fk_assessment_versions_assessment
    FOREIGN KEY (assessment_id)
    REFERENCES assessments (id)
    ON UPDATE RESTRICT
    ON DELETE RESTRICT,

    CONSTRAINT fk_assessment_versions_created_by
        FOREIGN KEY (created_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_assessment_versions_assessment_status (
        assessment_id,
        status
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

ALTER TABLE assessments
    ADD COLUMN current_version_id BIGINT UNSIGNED NULL
        AFTER status,

    ADD CONSTRAINT fk_assessments_current_version
        FOREIGN KEY (current_version_id)
        REFERENCES assessment_versions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    ADD CONSTRAINT chk_assessments_current_version
        CHECK (
            (status = 'draft' AND current_version_id IS NULL)
            OR
            (
                status IN ('published', 'archived')
                AND current_version_id IS NOT NULL
            )
        ),

    ADD INDEX idx_assessments_current_version (
        current_version_id
    );