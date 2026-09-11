CREATE TABLE attempt_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    attempt_id BIGINT UNSIGNED NOT NULL,

    event_type VARCHAR(64) NOT NULL,

    actor_user_id BIGINT UNSIGNED NULL,

    from_status VARCHAR(32) NULL,
    to_status VARCHAR(32) NULL,

    details_json JSON NULL,

    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (id),

    CONSTRAINT chk_attempt_events_from_status
        CHECK (
            from_status IS NULL
            OR from_status IN (
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

    CONSTRAINT chk_attempt_events_to_status
        CHECK (
            to_status IS NULL
            OR to_status IN (
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

    CONSTRAINT chk_attempt_events_transition
        CHECK (
            from_status IS NULL
            OR to_status IS NULL
            OR from_status <> to_status
        ),

    CONSTRAINT fk_attempt_events_attempt
        FOREIGN KEY (attempt_id)
        REFERENCES attempts (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_attempt_events_actor
        FOREIGN KEY (actor_user_id)
        REFERENCES users (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    INDEX idx_attempt_events_attempt_time (
        attempt_id,
        created_at
    ),

    INDEX idx_attempt_events_type_time (
        event_type,
        created_at
    ),

    INDEX idx_attempt_events_actor_time (
        actor_user_id,
        created_at
    )
) ENGINE = InnoDB
  DEFAULT CHARACTER SET = utf8mb4
  COLLATE = utf8mb4_unicode_ci;