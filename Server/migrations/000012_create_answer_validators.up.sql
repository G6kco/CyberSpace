CREATE TABLE answer_validators (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    question_id BIGINT UNSIGNED NOT NULL,
    sequence_no SMALLINT UNSIGNED NOT NULL DEFAULT 1,

    validator_type VARCHAR(32) NOT NULL,

    expected_hmac BINARY(32) NULL,
    validator_config JSON NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_answer_validators_question_sequence
        UNIQUE (question_id, sequence_no),

    CONSTRAINT chk_answer_validators_type
        CHECK (
            validator_type IN (
                'exact_hmac',
                'case_insensitive_hmac',
                'regex',
                'numeric_range',
                'mcq_option',
                'manual',
                'dynamic_flag'
            )
        ),

    CONSTRAINT chk_answer_validators_configuration
        CHECK (
            (
                validator_type IN (
                    'exact_hmac',
                    'case_insensitive_hmac',
                    'mcq_option'
                )
                AND expected_hmac IS NOT NULL
            )
            OR
            (
                validator_type IN (
                    'regex',
                    'numeric_range'
                )
                AND validator_config IS NOT NULL
            )
            OR
            validator_type IN (
                'manual',
                'dynamic_flag'
            )
        ),

    CONSTRAINT fk_answer_validators_question
        FOREIGN KEY (question_id)
        REFERENCES assessment_questions (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_answer_validators_question (
        question_id
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;