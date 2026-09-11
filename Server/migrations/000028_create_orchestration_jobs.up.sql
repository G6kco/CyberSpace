CREATE TABLE orchestration_jobs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    lab_instance_id BIGINT UNSIGNED NOT NULL,

    job_type VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',

    idempotency_key VARCHAR(128)
        CHARACTER SET ascii
        COLLATE ascii_bin
        NOT NULL,

    payload_json JSON NULL,

    attempt_count SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    max_attempts SMALLINT UNSIGNED NOT NULL DEFAULT 3,

    available_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    locked_at DATETIME(6) NULL,
    locked_by VARCHAR(128) NULL,

    completed_at DATETIME(6) NULL,
    last_error MEDIUMTEXT NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_orchestration_jobs_idempotency
        UNIQUE (idempotency_key),

    CONSTRAINT chk_orchestration_jobs_type
        CHECK (
            job_type IN (
                'provision',
                'stop',
                'reset',
                'cleanup'
            )
        ),

    CONSTRAINT chk_orchestration_jobs_status
        CHECK (
            status IN (
                'pending',
                'running',
                'succeeded',
                'failed',
                'cancelled'
            )
        ),

    CONSTRAINT chk_orchestration_jobs_attempts
        CHECK (
            max_attempts > 0
            AND attempt_count <= max_attempts
        ),

    CONSTRAINT chk_orchestration_jobs_lock
        CHECK (
            (
                locked_at IS NULL
                AND locked_by IS NULL
            )
            OR
            (
                locked_at IS NOT NULL
                AND locked_by IS NOT NULL
            )
        ),

    CONSTRAINT chk_orchestration_jobs_completion
        CHECK (
            (
                status IN ('succeeded', 'failed', 'cancelled')
                AND completed_at IS NOT NULL
            )
            OR
            (
                status IN ('pending', 'running')
                AND completed_at IS NULL
            )
        ),

    CONSTRAINT fk_orchestration_jobs_instance
        FOREIGN KEY (lab_instance_id)
        REFERENCES lab_instances (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    INDEX idx_orchestration_jobs_worker (
        status,
        available_at
    ),

    INDEX idx_orchestration_jobs_instance (
        lab_instance_id,
        created_at
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;