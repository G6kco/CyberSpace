-- Rolling back fails while an attempt holds more than one lab or a scenario
-- question exists: the restored constraints cannot hold for that data.
ALTER TABLE lab_instances
    ADD CONSTRAINT uq_lab_instances_one_active
        UNIQUE (active_attempt_id),
    ADD CONSTRAINT uq_lab_instances_attempt_generation
        UNIQUE (attempt_id, generation_no);

ALTER TABLE lab_instances
    DROP FOREIGN KEY fk_lab_instances_question,
    DROP INDEX uq_lab_instances_active_slot,
    DROP INDEX uq_lab_instances_slot_generation,
    DROP INDEX fk_lab_instances_question,
    DROP COLUMN active_slot,
    DROP COLUMN lab_slot,
    DROP COLUMN question_id;

DROP TABLE question_files;

ALTER TABLE assessment_questions
    DROP CHECK chk_assessment_questions_pool,
    DROP CHECK chk_assessment_questions_pool_material;

ALTER TABLE assessment_questions
    ADD CONSTRAINT chk_assessment_questions_pool
        CHECK (pool IS NULL OR pool IN ('wireshark', 'nmap')),
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
        );

ALTER TABLE attempts
    DROP CHECK chk_attempts_pass_score,
    DROP COLUMN pass_score;

ALTER TABLE assessment_versions
    DROP CHECK chk_assessment_versions_dealing,
    DROP COLUMN pass_percent,
    DROP COLUMN questions_per_attempt;
