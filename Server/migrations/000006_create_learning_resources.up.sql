CREATE TABLE learning_resources (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    module_id BIGINT UNSIGNED NOT NULL,

    resource_type VARCHAR(32) NOT NULL,
    title VARCHAR(200) NOT NULL,

    content MEDIUMTEXT NULL,
    external_url VARCHAR(2048) NULL,
    metadata_json JSON NULL,

    sequence_no SMALLINT UNSIGNED NOT NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_learning_resources_module_sequence
        UNIQUE (module_id, sequence_no),

    CONSTRAINT chk_learning_resources_type
        CHECK (
            resource_type IN (
                'markdown',
                'video',
                'pdf',
                'link',
                'file'
            )
        ),

    CONSTRAINT chk_learning_resources_source
        CHECK (
            content IS NOT NULL
            OR external_url IS NOT NULL
        ),

    CONSTRAINT fk_learning_resources_module
        FOREIGN KEY (module_id)
        REFERENCES learning_modules (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_learning_resources_module (
        module_id,
        sequence_no
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;