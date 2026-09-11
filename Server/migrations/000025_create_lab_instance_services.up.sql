CREATE TABLE lab_instance_services (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    lab_instance_id BIGINT UNSIGNED NOT NULL,

    service_name VARCHAR(128) NOT NULL,

    container_id VARCHAR(128)
        CHARACTER SET ascii
        COLLATE ascii_bin
        NULL,

    state VARCHAR(32) NOT NULL DEFAULT 'pending',

    internal_hostname VARCHAR(253)
        CHARACTER SET ascii
        COLLATE ascii_bin
        NULL,

    published_ports_json JSON NULL,

    started_at DATETIME(6) NULL,
    stopped_at DATETIME(6) NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_lab_instance_services_name
        UNIQUE (lab_instance_id, service_name),

    CONSTRAINT uq_lab_instance_services_container
        UNIQUE (container_id),

    CONSTRAINT chk_lab_instance_services_state
        CHECK (
            state IN (
                'pending',
                'creating',
                'running',
                'stopped',
                'failed',
                'removed'
            )
        ),

    CONSTRAINT chk_lab_instance_services_times
        CHECK (
            stopped_at IS NULL
            OR (
                started_at IS NOT NULL
                AND stopped_at >= started_at
            )
        ),

    CONSTRAINT chk_lab_instance_services_running
        CHECK (
            state <> 'running'
            OR (
                container_id IS NOT NULL
                AND started_at IS NOT NULL
                AND stopped_at IS NULL
            )
        ),

    CONSTRAINT fk_lab_instance_services_instance
        FOREIGN KEY (lab_instance_id)
        REFERENCES lab_instances (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_lab_instance_services_instance_state (
        lab_instance_id,
        state
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;