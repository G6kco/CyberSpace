ALTER TABLE assessment_questions
    DROP FOREIGN KEY fk_assessment_questions_lab_template,
    DROP CHECK chk_assessment_questions_pool,
    DROP CHECK chk_assessment_questions_difficulty,
    DROP CHECK chk_assessment_questions_pool_material,
    DROP CHECK chk_assessment_questions_pcap_file,
    DROP INDEX idx_assessment_questions_version_pool,
    DROP INDEX fk_assessment_questions_lab_template,
    DROP COLUMN pcap_file,
    DROP COLUMN lab_template_version_id,
    DROP COLUMN difficulty,
    DROP COLUMN pool;
