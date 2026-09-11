CREATE TABLE question_options (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    question_id BIGINT UNSIGNED NOT NULL,

    option_key VARCHAR(32) NOT NULL,
    label TEXT NOT NULL,
    sequence_no SMALLINT UNSIGNED NOT NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_question_options_key
        UNIQUE (question_id, option_key),

    CONSTRAINT uq_question_options_sequence
        UNIQUE (question_id, sequence_no),

    CONSTRAINT fk_question_options_question
        FOREIGN KEY (question_id)
        REFERENCES assessment_questions (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_question_options_question (
        question_id,
        sequence_no
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;