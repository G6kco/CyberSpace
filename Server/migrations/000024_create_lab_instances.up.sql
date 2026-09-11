CREATE TABLE lab_instances (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    attempt_id BIGINT UNSIGNED NOT NULL,
    lab_template_version_id BIGINT UNSIGNED NOT NULL,
    docker_host_id BIGINT UNSIGNED NOT NULL,

    generation_no SMALLINT UNSIGNED NOT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'queued',

    started_at DATETIME(6) NULL,
    ready_at DATETIME(6) NULL,
    stopped_at DATETIME(6) NULL,

    failure_code VARCHAR(64) NULL,
    failure_message VARCHAR(1000) NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    active_attempt_id BIGINT UNSIGNED
        GENERATED ALWAYS AS (
            CASE
                WHEN status IN (
                    'queued',
                    'provisioning',
                    'running',
                    'stopping'
                )
                THEN attempt_id
                ELSE NULL
            END
        ) STORED,

    PRIMARY KEY (id),

    CONSTRAINT uq_lab_instances_attempt_generation
        UNIQUE (attempt_id, generation_no),

    CONSTRAINT uq_lab_instances_one_active
        UNIQUE (active_attempt_id),

    CONSTRAINT chk_lab_instances_generation
        CHECK (generation_no > 0),

    CONSTRAINT chk_lab_instances_status
        CHECK (
            status IN (
                'queued',
                'provisioning',
                'running',
                'stopping',
                'stopped',
                'failed'
            )
        ),

    CONSTRAINT chk_lab_instances_ready_time
        CHECK (
            ready_at IS NULL
            OR (
                started_at IS NOT NULL
                AND ready_at >= started_at
            )
        ),

    CONSTRAINT chk_lab_instances_stopped_time
        CHECK (
            stopped_at IS NULL
            OR stopped_at >= COALESCE(
                ready_at,
                started_at,
                created_at
            )
        ),

    CONSTRAINT chk_lab_instances_running
        CHECK (
            status <> 'running'
            OR (
                started_at IS NOT NULL
                AND ready_at IS NOT NULL
                AND stopped_at IS NULL
            )
        ),

    CONSTRAINT chk_lab_instances_finished
        CHECK (
            status NOT IN ('stopped', 'failed')
            OR stopped_at IS NOT NULL
        ),

    CONSTRAINT chk_lab_instances_failure
        CHECK (
            (
                status = 'failed'
                AND failure_code IS NOT NULL
                AND failure_message IS NOT NULL
            )
            OR
            (
                status <> 'failed'
                AND failure_code IS NULL
                AND failure_message IS NULL
            )
        ),

    CONSTRAINT fk_lab_instances_attempt
    FOREIGN KEY (attempt_id)
    REFERENCES attempts (id)
    ON UPDATE RESTRICT
    ON DELETE RESTRICT,

    CONSTRAINT fk_lab_instances_template_version
        FOREIGN KEY (lab_template_version_id)
        REFERENCES lab_template_versions (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_lab_instances_docker_host
        FOREIGN KEY (docker_host_id)
        REFERENCES docker_hosts (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_lab_instances_host_status (
        docker_host_id,
        status
    ),

    INDEX idx_lab_instances_status_created (
        status,
        created_at
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;