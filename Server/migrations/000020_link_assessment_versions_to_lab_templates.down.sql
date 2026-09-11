ALTER TABLE assessment_versions
    DROP FOREIGN KEY fk_assessment_versions_lab_template,
    DROP INDEX idx_assessment_versions_lab_template,
    DROP COLUMN lab_template_version_id;