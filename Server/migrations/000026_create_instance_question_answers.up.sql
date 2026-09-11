CREATE TABLE instance_question_answers (
    lab_instance_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,

    expected_hmac BINARY(32) NOT NULL,
    expires_at DATETIME(6) NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (
        lab_instance_id,
        question_id
    ),

    CONSTRAINT chk_instance_question_answers_expiry
        CHECK (
            expires_at IS NULL
            OR expires_at > created_at
        ),

    CONSTRAINT fk_instance_question_answers_instance
        FOREIGN KEY (lab_instance_id)
        REFERENCES lab_instances (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_instance_question_answers_question
        FOREIGN KEY (question_id)
        REFERENCES assessment_questions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_instance_question_answers_question (
        question_id
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;