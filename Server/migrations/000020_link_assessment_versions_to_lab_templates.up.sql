ALTER TABLE assessment_versions
    ADD COLUMN lab_template_version_id BIGINT UNSIGNED NULL
        AFTER max_attempts,

    ADD CONSTRAINT fk_assessment_versions_lab_template
        FOREIGN KEY (lab_template_version_id)
        REFERENCES lab_template_versions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    ADD INDEX idx_assessment_versions_lab_template (
        lab_template_version_id
    );