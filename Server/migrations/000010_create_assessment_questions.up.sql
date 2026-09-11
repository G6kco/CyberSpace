CREATE TABLE assessment_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    assessment_version_id BIGINT UNSIGNED NOT NULL,
    parent_question_id BIGINT UNSIGNED NULL,

    code VARCHAR(64) NOT NULL,
    prompt MEDIUMTEXT NOT NULL,

    answer_type VARCHAR(32) NOT NULL,

    points DECIMAL(10,2) UNSIGNED NOT NULL DEFAULT 0.00,
    max_submissions SMALLINT UNSIGNED NULL,

    sequence_no SMALLINT UNSIGNED NOT NULL,
    is_required BOOLEAN NOT NULL DEFAULT TRUE,

    metadata_json JSON NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_assessment_questions_version_code
        UNIQUE (assessment_version_id, code),

    CONSTRAINT uq_assessment_questions_id_version
        UNIQUE (id, assessment_version_id),

    CONSTRAINT chk_assessment_questions_answer_type
        CHECK (
            answer_type IN (
                'group',
                'flag',
                'text',
                'mcq',
                'numeric',
                'manual'
            )
        ),

    CONSTRAINT chk_assessment_questions_points
        CHECK (
            (answer_type = 'group' AND points = 0.00)
            OR
            (answer_type <> 'group' AND points > 0.00)
        ),

    CONSTRAINT chk_assessment_questions_submissions
        CHECK (
            max_submissions IS NULL
            OR max_submissions > 0
        ),

    CONSTRAINT fk_assessment_questions_version
        FOREIGN KEY (assessment_version_id)
        REFERENCES assessment_versions (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_assessment_questions_parent
        FOREIGN KEY (
            parent_question_id,
            assessment_version_id
        )
        REFERENCES assessment_questions (
            id,
            assessment_version_id
        )
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_assessment_questions_version_order (
        assessment_version_id,
        parent_question_id,
        sequence_no
    ),

    INDEX idx_assessment_questions_parent_version (
        parent_question_id,
        assessment_version_id
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;