CREATE TABLE attempts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    access_grant_id BIGINT UNSIGNED NOT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'ready',

    started_by BIGINT UNSIGNED NULL,
    started_at DATETIME(6) NULL,
    deadline_at DATETIME(6) NULL,

    submitted_at DATETIME(6) NULL,
    ended_at DATETIME(6) NULL,

    termination_reason VARCHAR(500) NULL,

    score_awarded DECIMAL(10,2) UNSIGNED NOT NULL DEFAULT 0.00,
    max_score DECIMAL(10,2) UNSIGNED NOT NULL,

    result VARCHAR(32) NOT NULL DEFAULT 'pending',

    lock_version INT UNSIGNED NOT NULL DEFAULT 0,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL
        DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT uq_attempts_access_grant
        UNIQUE (access_grant_id),

    CONSTRAINT chk_attempts_status
        CHECK (
            status IN (
                'ready',
                'starting',
                'in_progress',
                'submitted',
                'expired',
                'revoked',
                'failed',
                'evaluated'
            )
        ),

    CONSTRAINT chk_attempts_result
        CHECK (
            result IN (
                'pending',
                'passed',
                'failed'
            )
        ),

    CONSTRAINT chk_attempts_score
        CHECK (
            max_score > 0.00
            AND score_awarded <= max_score
        ),

    CONSTRAINT chk_attempts_started_by
        CHECK (
            started_at IS NULL
            OR started_by IS NOT NULL
        ),

    CONSTRAINT chk_attempts_deadline
        CHECK (
            deadline_at IS NULL
            OR (
                started_at IS NOT NULL
                AND deadline_at > started_at
            )
        ),

    CONSTRAINT chk_attempts_submission_time
        CHECK (
            submitted_at IS NULL
            OR (
                started_at IS NOT NULL
                AND submitted_at >= started_at
            )
        ),

    CONSTRAINT chk_attempts_end_time
        CHECK (
            ended_at IS NULL
            OR ended_at >= COALESCE(started_at, created_at)
        ),

    CONSTRAINT chk_attempts_ready_state
        CHECK (
            status <> 'ready'
            OR (
                started_by IS NULL
                AND started_at IS NULL
                AND deadline_at IS NULL
                AND submitted_at IS NULL
                AND ended_at IS NULL
            )
        ),

    CONSTRAINT chk_attempts_starting_state
        CHECK (
            status <> 'starting'
            OR (
                started_by IS NOT NULL
                AND started_at IS NULL
                AND deadline_at IS NULL
                AND ended_at IS NULL
            )
        ),

    CONSTRAINT chk_attempts_in_progress_state
        CHECK (
            status <> 'in_progress'
            OR (
                started_by IS NOT NULL
                AND started_at IS NOT NULL
                AND deadline_at IS NOT NULL
                AND ended_at IS NULL
            )
        ),

    CONSTRAINT chk_attempts_submitted_state
        CHECK (
            status <> 'submitted'
            OR (
                submitted_at IS NOT NULL
                AND ended_at IS NOT NULL
            )
        ),

    CONSTRAINT chk_attempts_terminal_state
        CHECK (
            status NOT IN (
                'expired',
                'revoked',
                'failed',
                'evaluated'
            )
            OR ended_at IS NOT NULL
        ),

    CONSTRAINT chk_attempts_submission_status
        CHECK (
            submitted_at IS NULL
            OR status IN ('submitted', 'evaluated')
        ),

    CONSTRAINT chk_attempts_termination_reason
        CHECK (
            status NOT IN ('revoked', 'failed')
            OR (
                termination_reason IS NOT NULL
                AND CHAR_LENGTH(TRIM(termination_reason)) > 0
            )
        ),

    CONSTRAINT chk_attempts_evaluation
        CHECK (
            (
                status = 'evaluated'
                AND result IN ('passed', 'failed')
            )
            OR
            (
                status <> 'evaluated'
                AND result = 'pending'
            )
        ),

    CONSTRAINT fk_attempts_access_grant
        FOREIGN KEY (access_grant_id)
        REFERENCES assessment_access_grants (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_attempts_started_by
        FOREIGN KEY (started_by)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_attempts_status_deadline (
        status,
        deadline_at
    ),

    INDEX idx_attempts_started_by (
        started_by
    ),

    INDEX idx_attempts_created_at (
        created_at
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;