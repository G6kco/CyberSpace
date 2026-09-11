ALTER TABLE assessments
    DROP FOREIGN KEY fk_assessments_current_version,
    DROP CHECK chk_assessments_current_version,
    DROP INDEX idx_assessments_current_version,
    DROP COLUMN current_version_id;

DROP TABLE IF EXISTS assessment_versions;