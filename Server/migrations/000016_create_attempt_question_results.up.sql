CREATE TABLE attempt_question_results (
    attempt_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,

    best_score DECIMAL(10,2) UNSIGNED NOT NULL DEFAULT 0.00,
    is_solved BOOLEAN NOT NULL DEFAULT FALSE,

    solved_at DATETIME(6) NULL,
    last_submission_id BIGINT UNSIGNED NULL,

    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (
        attempt_id,
        question_id
    ),

    CONSTRAINT chk_attempt_question_results_solved
        CHECK (
            (
                is_solved = FALSE
                AND solved_at IS NULL
            )
            OR
            (
                is_solved = TRUE
                AND solved_at IS NOT NULL
            )
        ),

    CONSTRAINT fk_attempt_question_results_attempt
        FOREIGN KEY (attempt_id)
        REFERENCES attempts (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_attempt_question_results_question
        FOREIGN KEY (question_id)
        REFERENCES assessment_questions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_attempt_question_results_submission
        FOREIGN KEY (
            last_submission_id,
            attempt_id,
            question_id
        )
        REFERENCES question_submissions (
            id,
            attempt_id,
            question_id
        )
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_attempt_question_results_attempt_solved (
        attempt_id,
        is_solved
    ),

    INDEX idx_attempt_question_results_question (
        question_id
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci; 