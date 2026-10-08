-- Level assessments deal each student a random question from each pool, so a
-- top-level question records its pool and the material it needs: a capture
-- file for Wireshark, a lab image for Nmap. Child questions (flags and
-- sub-questions) leave these NULL.
ALTER TABLE assessment_questions
    ADD COLUMN pool VARCHAR(32) NULL
        AFTER answer_type,
    ADD COLUMN difficulty VARCHAR(16) NULL
        AFTER pool,
    ADD COLUMN lab_template_version_id BIGINT UNSIGNED NULL
        AFTER difficulty,
    ADD COLUMN pcap_file VARCHAR(255) NULL
        AFTER lab_template_version_id,
    ADD CONSTRAINT chk_assessment_questions_pool
        CHECK (pool IS NULL OR pool IN ('wireshark', 'nmap')),
    ADD CONSTRAINT chk_assessment_questions_difficulty
        CHECK (difficulty IS NULL OR difficulty IN ('easy', 'hard')),
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
        ),
    -- The API joins pcap_file onto the capture directory, so it must be a
    -- bare file name.
    ADD CONSTRAINT chk_assessment_questions_pcap_file
        CHECK (
            pcap_file IS NULL
            OR (
                CHAR_LENGTH(TRIM(pcap_file)) > 0
                AND pcap_file NOT LIKE '%/%'
                AND pcap_file NOT LIKE '%\\%'
                AND pcap_file NOT LIKE '%..%'
            )
        ),
    ADD CONSTRAINT fk_assessment_questions_lab_template
        FOREIGN KEY (lab_template_version_id)
        REFERENCES lab_template_versions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,
    ADD INDEX idx_assessment_questions_version_pool (
        assessment_version_id,
        pool
    );
