-- Levels built by administrators. Unlike Level 1, which deals one question
-- from each pool, a built level deals questions_per_attempt random questions
-- from its bank. Each question is a scenario with its own optional lab and
-- downloadable files, and marks differ between questions, so the pass mark
-- is a percentage of what the student was dealt.
ALTER TABLE assessment_versions
    ADD COLUMN questions_per_attempt SMALLINT UNSIGNED NULL
        AFTER max_attempts,
    ADD COLUMN pass_percent DECIMAL(5,2) UNSIGNED NULL
        AFTER pass_score,
    ADD CONSTRAINT chk_assessment_versions_dealing
        CHECK (
            (questions_per_attempt IS NULL AND pass_percent IS NULL)
            OR
            (
                questions_per_attempt > 0
                AND pass_percent > 0.00
                AND pass_percent <= 100.00
            )
        );

-- An attempt of a built level stores its own pass mark, computed from the
-- marks of the questions dealt to it. NULL means the version's pass_score.
ALTER TABLE attempts
    ADD COLUMN pass_score DECIMAL(10,2) UNSIGNED NULL
        AFTER max_score,
    ADD CONSTRAINT chk_attempts_pass_score
        CHECK (pass_score IS NULL OR pass_score <= max_score);

-- A 'scenario' question belongs to a built level: its lab is optional and
-- its files live in question_files rather than pcap_file.
ALTER TABLE assessment_questions
    DROP CHECK chk_assessment_questions_pool,
    DROP CHECK chk_assessment_questions_pool_material;

ALTER TABLE assessment_questions
    ADD CONSTRAINT chk_assessment_questions_pool
        CHECK (pool IS NULL OR pool IN ('wireshark', 'nmap', 'scenario')),
    ADD CONSTRAINT chk_assessment_questions_pool_material
        CHECK (
            (
                pool IS NULL
                AND difficulty IS NULL
                AND lab_template_version_id IS NULL
                AND pcap_file IS NULL
            )
            OR
            (
                pool = 'wireshark'
                AND parent_question_id IS NULL
                AND difficulty IS NOT NULL
                AND lab_template_version_id IS NULL
                AND pcap_file IS NOT NULL
            )
            OR
            (
                pool = 'nmap'
                AND parent_question_id IS NULL
                AND difficulty IS NOT NULL
                AND lab_template_version_id IS NOT NULL
                AND pcap_file IS NULL
            )
            OR
            (
                pool = 'scenario'
                AND parent_question_id IS NULL
                AND pcap_file IS NULL
            )
        );

-- Files a student downloads for a question. stored_name is the random name
-- on disk; the API never joins a user-supplied name onto the directory.
CREATE TABLE question_files (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    question_id BIGINT UNSIGNED NOT NULL,

    stored_name VARCHAR(64)
        CHARACTER SET ascii
        COLLATE ascii_bin
        NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    size_bytes BIGINT UNSIGNED NOT NULL,
    sha256 BINARY(32) NOT NULL,

    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_question_files_stored_name
        UNIQUE (stored_name),

    CONSTRAINT chk_question_files_stored_name
        CHECK (stored_name REGEXP '^[0-9a-f]{32}$'),

    CONSTRAINT chk_question_files_original_name
        CHECK (CHAR_LENGTH(TRIM(original_name)) > 0),

    CONSTRAINT fk_question_files_question
        FOREIGN KEY (question_id)
        REFERENCES assessment_questions (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_question_files_created_by
        FOREIGN KEY (created_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_question_files_question (
        question_id
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;

-- Each dealt question can have its own container, so an attempt may hold
-- several active labs: one per question. lab_slot is the question, or 0 for
-- labs requested before this migration. The new unique keys lead with
-- attempt_id, so they can back fk_lab_instances_attempt once the old ones go.
ALTER TABLE lab_instances
    ADD COLUMN question_id BIGINT UNSIGNED NULL
        AFTER attempt_id,
    ADD COLUMN lab_slot BIGINT UNSIGNED
        GENERATED ALWAYS AS (COALESCE(question_id, 0)) STORED
        AFTER question_id,
    ADD COLUMN active_slot BIGINT UNSIGNED
        GENERATED ALWAYS AS (
            CASE
                WHEN status IN (
                    'queued',
                    'provisioning',
                    'running',
                    'stopping'
                )
                THEN COALESCE(question_id, 0)
                ELSE NULL
            END
        ) STORED,
    ADD CONSTRAINT uq_lab_instances_active_slot
        UNIQUE (active_attempt_id, active_slot),
    ADD CONSTRAINT uq_lab_instances_slot_generation
        UNIQUE (attempt_id, lab_slot, generation_no),
    ADD CONSTRAINT fk_lab_instances_question
        FOREIGN KEY (question_id)
        REFERENCES assessment_questions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT;

ALTER TABLE lab_instances
    DROP INDEX uq_lab_instances_one_active,
    DROP INDEX uq_lab_instances_attempt_generation;
